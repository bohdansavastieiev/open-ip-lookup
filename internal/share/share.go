// Package share stores temporary shared lookup inputs.
package share

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/ipinput"
	_ "modernc.org/sqlite"
)

const (
	DefaultTTL             = 7 * 24 * time.Hour
	DefaultCleanupInterval = time.Hour
	defaultBearerBytes     = 32
	defaultDirName         = "shares"
	defaultDBFileName      = "shares.sqlite"
	sqliteDriverName       = "sqlite"
)

const (
	pragmaBusyTimeoutSQL = "PRAGMA busy_timeout = 5000"
	pragmaForeignKeysSQL = "PRAGMA foreign_keys = ON"
	createSharesSQL      = `CREATE TABLE IF NOT EXISTS shares (
		id INTEGER PRIMARY KEY,
		bearer_hash BLOB NOT NULL UNIQUE,
		input TEXT NOT NULL CHECK(input <> ''),
		created_at INTEGER NOT NULL,
		expires_at INTEGER NOT NULL,
		last_visited_at INTEGER,
		visit_count INTEGER NOT NULL DEFAULT 0 CHECK(visit_count >= 0)
	)`
	createSharesExpiresAtIndexSQL = "CREATE INDEX IF NOT EXISTS " +
		"shares_expires_at_idx ON shares(expires_at)"
	insertShareSQL = `
		INSERT INTO shares (bearer_hash, input, created_at, expires_at)
		VALUES (?, ?, ?, ?)
	`
	selectShareByBearerSQL = `
		SELECT id, input, expires_at, visit_count
		FROM shares
		WHERE bearer_hash = ?
	`
	deleteShareByIDSQL     = "DELETE FROM shares WHERE id = ?"
	updateResolvedShareSQL = `
		UPDATE shares
		SET expires_at = ?, last_visited_at = ?, visit_count = ?
		WHERE id = ?
	`
	deleteExpiredSharesSQL = "DELETE FROM shares WHERE expires_at <= ?"
)

var initStatements = []string{
	pragmaBusyTimeoutSQL,
	pragmaForeignKeysSQL,
	createSharesSQL,
	createSharesExpiresAtIndexSQL,
}

var (
	ErrNoIPs    = errors.New("no IP addresses found")
	ErrNotFound = errors.New("share not found")
)

type Store struct {
	db          *sql.DB
	ttl         time.Duration
	bearerBytes int
	now         func() time.Time
}

type Created struct {
	ID        int64
	Bearer    string
	ExpiresAt time.Time
}

type Resolved struct {
	ID         int64
	Input      string
	ExpiresAt  time.Time
	VisitCount int
}

func DefaultDBPath(dataDir string) string {
	return filepath.Join(dataDir, defaultDirName, defaultDBFileName)
}

func Open(path string) (*Store, error) {
	if err := ensureDBDir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	if err := ensureDBFile(path); err != nil {
		return nil, err
	}

	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	s := &Store{
		db:          db,
		ttl:         DefaultTTL,
		bearerBytes: defaultBearerBytes,
		now:         time.Now,
	}
	if err := s.init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func ensureDBDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create share db dir: %w", err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("restrict share db dir: %w", err)
	}
	return nil
}

func ensureDBFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("open share db file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close share db file: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("restrict share db file: %w", err)
	}
	return nil
}

func (s *Store) init(ctx context.Context) error {
	for _, statement := range initStatements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Create(ctx context.Context, raw string) (Created, error) {
	input := canonicalInput(raw)
	if input == "" {
		return Created{}, ErrNoIPs
	}

	bearer, err := generateBearer(s.bearerBytes)
	if err != nil {
		return Created{}, err
	}

	now := s.now().UTC().Truncate(time.Second)
	expiresAt := now.Add(s.ttl)
	result, err := s.db.ExecContext(
		ctx,
		insertShareSQL,
		bearerHash(bearer),
		input,
		now.Unix(),
		expiresAt.Unix(),
	)
	if err != nil {
		return Created{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Created{}, err
	}
	return Created{ID: id, Bearer: bearer, ExpiresAt: expiresAt}, nil
}

func (s *Store) Resolve(ctx context.Context, bearer string) (Resolved, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Resolved{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var id int64
	var input string
	var expiresAtUnix int64
	var visitCount int
	err = tx.QueryRowContext(ctx, selectShareByBearerSQL, bearerHash(bearer)).Scan(
		&id,
		&input,
		&expiresAtUnix,
		&visitCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Resolved{}, ErrNotFound
	}
	if err != nil {
		return Resolved{}, err
	}

	now := s.now().UTC().Truncate(time.Second)
	if !time.Unix(expiresAtUnix, 0).UTC().After(now) {
		if _, err := tx.ExecContext(ctx, deleteShareByIDSQL, id); err != nil {
			return Resolved{}, err
		}
		if err := tx.Commit(); err != nil {
			return Resolved{}, err
		}
		return Resolved{}, ErrNotFound
	}

	expiresAt := now.Add(s.ttl)
	visitCount++
	_, err = tx.ExecContext(
		ctx,
		updateResolvedShareSQL,
		expiresAt.Unix(),
		now.Unix(),
		visitCount,
		id,
	)
	if err != nil {
		return Resolved{}, err
	}
	if err := tx.Commit(); err != nil {
		return Resolved{}, err
	}

	return Resolved{
		ID:         id,
		Input:      input,
		ExpiresAt:  expiresAt,
		VisitCount: visitCount,
	}, nil
}

func (s *Store) DeleteExpired(ctx context.Context) (int64, error) {
	now := s.now().UTC().Truncate(time.Second)
	result, err := s.db.ExecContext(ctx, deleteExpiredSharesSQL, now.Unix())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) Close() error {
	return s.db.Close()
}

func canonicalInput(raw string) string {
	ips := ipinput.Parse(raw)
	if len(ips) == 0 {
		return ""
	}

	lines := make([]string, 0, len(ips))
	for _, ip := range ips {
		lines = append(lines, ip.String())
	}
	return strings.Join(lines, "\n")
}

func generateBearer(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func bearerHash(bearer string) []byte {
	sum := sha256.Sum256([]byte(bearer))
	return sum[:]
}
