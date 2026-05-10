// Package update is responsible for keeping the sources up to date and saving them to local dir
package update

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/config"
)

const (
	sourceWorkerLimit           = 4
	sourceRequestTimeout        = 15 * time.Minute
	sourceResponseHeaderTimeout = 15 * time.Second
	firstShortRetryDelay        = 10 * time.Second
	secondShortRetryDelay       = 30 * time.Second
)

var shortRetryDelays = []time.Duration{firstShortRetryDelay, secondShortRetryDelay}

type Updater struct {
	cfg        config.SourcesConfig
	logger     *slog.Logger
	httpClient *http.Client
}

func New(cfg config.SourcesConfig, logger *slog.Logger) *Updater {
	return &Updater{
		cfg:        cfg,
		logger:     logger,
		httpClient: newSourceHTTPClient(),
	}
}

func (u *Updater) Run(ctx context.Context, events chan<- SyncEvent) error {
	now := time.Now().UTC()
	s, err := loadStateForSync(u.cfg.DataDir, u.cfg.FullSync)
	if err != nil {
		return err
	}

	startupEvent, err := u.runStartup(ctx, now, &s)
	if err != nil {
		return err
	}
	if err := sendSyncEvent(ctx, events, startupEvent); err != nil {
		return err
	}

	for {
		if err := u.waitForNextSync(ctx, s.SyncSchedule); err != nil {
			return err
		}

		event, ok, err := u.runScheduled(ctx, &s)
		if err != nil {
			return err
		}
		if ok {
			if err := sendSyncEvent(ctx, events, event); err != nil {
				return err
			}
		}
	}
}

func newSourceHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   sourceResponseHeaderTimeout,
		KeepAlive: 30 * time.Second,
	}
	return &http.Client{
		Timeout: sourceRequestTimeout,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   sourceResponseHeaderTimeout,
			ResponseHeaderTimeout: sourceResponseHeaderTimeout,
			ExpectContinueTimeout: time.Second,
		},
	}
}

func (u *Updater) waitForNextSync(ctx context.Context, schedule syncSchedule) error {
	timer := time.NewTimer(time.Until(schedule.nextSyncAt()))
	defer stopTimer(timer)

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func stopTimer(timer *time.Timer) {
	if timer.Stop() {
		return
	}

	select {
	case <-timer.C:
	default:
	}
}

func (u *Updater) runScheduled(ctx context.Context, s *state) (SyncEvent, bool, error) {
	now := time.Now().UTC()
	if s.SyncSchedule.needsFullSync(now) {
		sourceIDs := filterOutdated(u.cfg.Enabled, s.Sources)
		event, err := u.updateSources(ctx, SyncScopeFull, sourceIDs, s, shortRetryDelays)
		return event, true, err
	}

	if s.SyncSchedule.needsRetrySync(now) {
		sourceIDs := u.refreshableFailedSourceIDs(s.Sources)
		event, err := u.updateSources(ctx, SyncScopePartial, sourceIDs, s, shortRetryDelays)
		return event, true, err
	}

	return SyncEvent{}, false, nil
}

func sendSyncEvent(ctx context.Context, events chan<- SyncEvent, event SyncEvent) error {
	select {
	case events <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
