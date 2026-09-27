package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	sanctionsNoticeFileName = "sanctions-notice-started"
	sanctionsNoticeDuration = 7 * 24 * time.Hour
)

// sanctionsNoticeUntil returns when the site stops announcing the sanctions flags. The first start
// of a release with this feature records its start time, so later deploys don't extend the notice.
func sanctionsNoticeUntil(dataDir string, now time.Time) (time.Time, error) {
	path := filepath.Join(dataDir, sanctionsNoticeFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		data = []byte(now.UTC().Format(time.RFC3339))
		err = os.WriteFile(path, data, 0o600)
	}
	if err != nil {
		return time.Time{}, err
	}

	startedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %q: %w", path, err)
	}
	return startedAt.Add(sanctionsNoticeDuration), nil
}
