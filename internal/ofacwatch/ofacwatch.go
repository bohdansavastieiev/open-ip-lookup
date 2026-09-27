// Package ofacwatch detects changes in US sanctions programs that may affect the OFAC flags in
// internal/dataset/ofac.go. It only notifies; the flag lists are reviewed and updated by hand.
package ofacwatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/notify"
)

const (
	checkInterval         = 24 * time.Hour
	requestTimeout        = 2 * time.Minute
	failureAlertThreshold = 3
	stateFileName         = "ofacwatch.json"
)

// anchor is the legal basis of a sanctions flag entry in internal/dataset/ofac.go.
type anchor struct {
	territory  string
	sdnProgram string
	cfrPart    string
}

var anchors = []anchor{
	{territory: "Cuba", sdnProgram: "CUBA", cfrPart: "515"},
	{territory: "Iran", sdnProgram: "IRAN", cfrPart: "560"},
	{territory: "North Korea", sdnProgram: "DPRK", cfrPart: "510"},
	{territory: "Crimea", sdnProgram: "UKRAINE-EO13685", cfrPart: "589"},
	{territory: "DNR and LNR", sdnProgram: "RUSSIA-EO14065"},
	{territory: "Russia", sdnProgram: "RUSSIA-EO14024", cfrPart: "587"},
	{territory: "Belarus", sdnProgram: "BELARUS-EO14038", cfrPart: "548"},
	{territory: "Venezuela", sdnProgram: "VENEZUELA-EO13850", cfrPart: "591"},
	{territory: "Burma", sdnProgram: "BURMA-EO14014", cfrPart: "525"},
}

type snapshot struct {
	SDNPrograms []string   `json:"sdn_programs"`
	CFRParts    []cfrPart  `json:"cfr_parts"`
	Documents   []document `json:"federal_register_documents"`
}

type cfrPart struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type document struct {
	Number string `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

type state struct {
	// Snapshot is nil until the first successful check.
	Snapshot            *snapshot `json:"snapshot"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
}

type Watcher struct {
	logger    *slog.Logger
	notifier  *notify.Notifier
	client    *http.Client
	statePath string
}

func New(dataDir string, notifier *notify.Notifier, logger *slog.Logger) *Watcher {
	return &Watcher{
		logger:    logger,
		notifier:  notifier,
		client:    &http.Client{Timeout: requestTimeout},
		statePath: filepath.Join(dataDir, stateFileName),
	}
}

func (w *Watcher) Run(ctx context.Context) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		w.check(ctx)
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

func (w *Watcher) check(ctx context.Context) {
	st, err := loadState(w.statePath)
	if err != nil {
		w.logger.Error("load ofac watch state", slog.Any("err", err))
		return
	}

	current, err := w.fetch(ctx)
	switch {
	case ctx.Err() != nil:
		return
	case err != nil:
		st.ConsecutiveFailures++
		w.logger.Warn(
			"ofac watch check failed",
			slog.Any("err", err),
			slog.Int("consecutive_failures", st.ConsecutiveFailures),
		)
		if st.ConsecutiveFailures == failureAlertThreshold {
			w.notifier.Notify(ctx, fmt.Sprintf(
				"OFAC watcher failed %d checks in a row, last error: %v", st.ConsecutiveFailures, err))
		}
	default:
		if lines := changes(st.Snapshot, current); len(lines) > 0 {
			w.notifier.Notify(ctx, "OFAC watcher: review internal/dataset/ofac.go\n"+
				strings.Join(lines, "\n"))
		}
		st = state{Snapshot: &current}
		w.logger.Info("ofac watch check completed")
	}

	if err := saveState(w.statePath, st); err != nil {
		w.logger.Error("save ofac watch state", slog.Any("err", err))
	}
}

// changes lists anchors that disappeared and new programs, regulation parts and documents.
// On the first check there is nothing to compare with, so only missing anchors are reported.
func changes(prev *snapshot, cur snapshot) []string {
	firstCheck := prev == nil
	if firstCheck {
		prev = &snapshot{}
	}
	curPrograms := keys(cur.SDNPrograms, func(p string) string { return p })
	prevPrograms := keys(prev.SDNPrograms, func(p string) string { return p })
	curParts := keys(cur.CFRParts, func(p cfrPart) string { return p.ID })
	prevParts := keys(prev.CFRParts, func(p cfrPart) string { return p.ID })

	var lines []string
	for _, a := range anchors {
		if gone(a.sdnProgram, curPrograms, prevPrograms, firstCheck) {
			lines = append(lines, fmt.Sprintf("%s: SDN program %s is gone", a.territory, a.sdnProgram))
		}
		if a.cfrPart != "" && gone(a.cfrPart, curParts, prevParts, firstCheck) {
			lines = append(lines, fmt.Sprintf("%s: 31 CFR part %s is gone", a.territory, a.cfrPart))
		}
	}
	if firstCheck {
		return lines
	}

	for _, p := range cur.SDNPrograms {
		if _, ok := prevPrograms[p]; !ok {
			lines = append(lines, "New SDN program: "+p)
		}
	}
	for _, p := range cur.CFRParts {
		if _, ok := prevParts[p.ID]; !ok {
			lines = append(lines, fmt.Sprintf("New 31 CFR part %s: %s", p.ID, p.Title))
		}
	}
	prevDocuments := keys(prev.Documents, func(d document) string { return d.Number })
	for _, d := range cur.Documents {
		if _, ok := prevDocuments[d.Number]; !ok {
			lines = append(lines, fmt.Sprintf("Federal Register: %s %s", d.Title, d.URL))
		}
	}
	return lines
}

func gone(key string, cur, prev map[string]struct{}, firstCheck bool) bool {
	_, inCur := cur[key]
	_, inPrev := prev[key]
	return !inCur && (firstCheck || inPrev)
}

func keys[T any](items []T, key func(T) string) map[string]struct{} {
	result := make(map[string]struct{}, len(items))
	for _, item := range items {
		result[key(item)] = struct{}{}
	}
	return result
}

func loadState(path string) (state, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return state{}, nil
	}
	if err != nil {
		return state{}, err
	}

	var st state
	if err := json.Unmarshal(data, &st); err != nil {
		return state{}, fmt.Errorf("decode %q: %w", path, err)
	}
	return st, nil
}

func saveState(path string, st state) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
