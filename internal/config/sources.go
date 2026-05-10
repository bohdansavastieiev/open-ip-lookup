package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
)

type SourcesConfig struct {
	DataDir  string         `json:"data_dir"`
	FullSync FullSyncConfig `json:"full_sync"`
	Enabled  []source.ID    `json:"enabled"`
}

type FullSyncConfig struct {
	IntervalHours int       `json:"interval_hours"`
	StartAtUTC    time.Time `json:"start_at_utc"`
}

const (
	minFullSyncIntervalHours = 1
	maxFullSyncIntervalHours = 168
)

func (c SourcesConfig) validate() error {
	var errs []error

	dataDirErr := validateRequired("sources.data_dir", c.DataDir)
	if dataDirErr != nil {
		errs = append(errs, dataDirErr)
	}

	if err := c.FullSync.validate(); err != nil {
		errs = append(errs, err)
	}

	hasEnabledErrors := false
	for i, id := range c.Enabled {
		if err := validateSourceID(fmt.Sprintf("sources.enabled[%d]", i), id); err != nil {
			errs = append(errs, err)
			hasEnabledErrors = true
		}
	}

	if !hasEnabledErrors {
		if err := validateUniqueEnabled(c.Enabled); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (c FullSyncConfig) validate() error {
	var errs []error

	if c.IntervalHours < minFullSyncIntervalHours ||
		c.IntervalHours > maxFullSyncIntervalHours {
		detail := fmt.Sprintf(
			"must be between %d and %d hours",
			minFullSyncIntervalHours,
			maxFullSyncIntervalHours,
		)
		errs = append(errs, New(
			"sources.full_sync.interval_hours",
			CodeRange,
			detail,
			c.IntervalHours,
		))
	}

	if c.StartAtUTC.IsZero() {
		errs = append(errs, New(
			"sources.full_sync.start_at_utc",
			CodeRequired,
			"must be set",
			c.StartAtUTC,
		))
	}

	return errors.Join(errs...)
}

func (c FullSyncConfig) Interval() time.Duration {
	return time.Duration(c.IntervalHours) * time.Hour
}

func validateSourceID(field string, id source.ID) error {
	if id.IsValid() {
		return nil
	}

	return New(
		field,
		CodeUnsupported,
		"this source ID is not recognized or supported by the application",
		string(id),
	)
}

func validateRequired(field, value string) error {
	if strings.TrimSpace(value) != value {
		return New(
			field,
			CodeWhitespace,
			"must not have leading or trailing whitespace",
			value,
		)
	}

	if value == "" {
		return New(field, CodeRequired, "must be set", value)
	}

	return nil
}

func validateUniqueEnabled(enabled []source.ID) error {
	var errs []error
	seen := make(map[source.ID]int, len(enabled))

	for i, id := range enabled {
		field := fmt.Sprintf("sources.enabled[%d]", i)
		if prev, ok := seen[id]; ok {
			detail := fmt.Sprintf("duplicates sources.enabled[%d]", prev)
			errs = append(errs, New(field, CodeDuplicate, detail, id))
		}
		seen[id] = i
	}

	return errors.Join(errs...)
}
