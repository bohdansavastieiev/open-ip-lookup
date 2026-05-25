// Package app orchestrates source sync, dataset loading, and server lifecycle.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/config"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/dataset"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/report"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/server"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/share"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/update"
)

type Manager struct {
	cfg    config.Config
	logger *slog.Logger

	mu         sync.RWMutex
	dataset    *dataset.Dataset
	hasMaxMind bool
	shares     *share.Store

	shareCleanupCancel context.CancelFunc
	shareCleanupDone   <-chan struct{}
}

func New(cfg config.Config, logger *slog.Logger) *Manager {
	return &Manager{cfg: cfg, logger: logger}
}

func (m *Manager) Run(ctx context.Context) error {
	if err := m.openShares(); err != nil {
		return err
	}
	m.logger.Info("share store opened")
	m.startShareCleanup(ctx)

	updater := update.New(m.cfg.Sources, m.logger)
	events := make(chan update.SyncEvent, 1)
	errCh := make(chan error, 2)
	go func() { errCh <- updater.Run(ctx, events) }()
	m.logger.Info("updater started")

	var srv *server.Server
	for {
		select {
		case event := <-events:
			serverStarted := srv != nil
			if !shouldLoadDataset(event, serverStarted) {
				continue
			}

			ds, err := m.loadDataset(event)
			if err != nil {
				if shouldKeepServingAfterLoadError(serverStarted) {
					m.logger.Info("dataset reload failed", slog.Any("err", err))
					continue
				}
				return errors.Join(err, m.shutdownServer(srv), m.Close())
			}

			if srv != nil {
				if err := m.switchDataset(ds, hasMaxMindSources(event.Available)); err != nil {
					return errors.Join(err, m.shutdownServer(srv), m.Close())
				}
				continue
			}

			m.dataset = ds
			m.hasMaxMind = hasMaxMindSources(event.Available)
			srv, err = server.New(m.cfg.Server, m, m.logger)
			if err != nil {
				_ = ds.Close()
				return errors.Join(err, m.Close())
			}
			m.startServer(srv, errCh)

		case err := <-errCh:
			if errors.Is(err, http.ErrServerClosed) {
				return m.Close()
			}
			return errors.Join(err, m.shutdownServer(srv), m.Close())

		case <-ctx.Done():
			return errors.Join(m.shutdownServer(srv), m.Close())
		}
	}
}

func (m *Manager) openShares() error {
	shares, err := share.Open(share.DefaultDBPath(m.cfg.Sources.DataDir))
	if err != nil {
		return err
	}
	m.shares = shares
	return nil
}

func (m *Manager) startShareCleanup(ctx context.Context) {
	cleanupCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	m.shareCleanupCancel = cancel
	m.shareCleanupDone = done

	go func() {
		defer close(done)
		m.cleanupShares(cleanupCtx)

		ticker := time.NewTicker(share.DefaultCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.cleanupShares(cleanupCtx)
			case <-cleanupCtx.Done():
				return
			}
		}
	}()
}

func (m *Manager) cleanupShares(ctx context.Context) {
	deleted, err := m.shares.DeleteExpired(ctx)
	if err != nil {
		m.logger.Error("delete expired shares", slog.Any("err", err))
		return
	}
	if deleted > 0 {
		m.logger.Info("deleted expired shares", slog.Int64("count", deleted))
	}
}

func (m *Manager) startServer(srv *server.Server, errCh chan<- error) {
	go func() {
		m.logger.Info("listening", slog.String("addr", m.cfg.Server.Addr))
		errCh <- srv.ListenAndServe()
	}()
}

func (m *Manager) shutdownServer(srv *server.Server) error {
	if srv == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(m.cfg.Server.ShutdownTimeoutSeconds)*time.Second,
	)
	defer cancel()

	return srv.Shutdown(ctx)
}

func (m *Manager) loadDataset(event update.SyncEvent) (*dataset.Dataset, error) {
	ds, err := dataset.Load(m.cfg.Sources.SourceDataDir(), event.Available, m.logger)
	if err != nil {
		return nil, err
	}

	m.logger.Info("dataset loaded", slog.Int("available", len(event.Available)))
	return ds, nil
}

func shouldLoadDataset(event update.SyncEvent, serverStarted bool) bool {
	if !serverStarted {
		return true
	}
	return len(event.Refreshed) > 0 || len(event.Outdated) > 0
}

func shouldKeepServingAfterLoadError(serverStarted bool) bool {
	return serverStarted
}

func (m *Manager) switchDataset(ds *dataset.Dataset, hasMaxMind bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	old := m.dataset
	m.dataset = ds
	m.hasMaxMind = hasMaxMind

	return old.Close()
}

func (m *Manager) HasMaxMind() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.hasMaxMind
}

func (m *Manager) Report(raw string) *report.Report {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return report.Get(raw, m.dataset)
}

func (m *Manager) LookupIP(ip netip.Addr) report.IPInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return report.InfoForIP(ip, m.dataset)
}

func (m *Manager) CreateShare(ctx context.Context, raw string) (share.Created, error) {
	return m.shares.Create(ctx, raw)
}

func (m *Manager) ResolveShare(ctx context.Context, bearer string) (share.Resolved, error) {
	return m.shares.Resolve(ctx, bearer)
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopShareCleanup()
	err := errors.Join(m.dataset.Close(), m.shares.Close())
	m.dataset = nil
	m.shares = nil

	return err
}

func (m *Manager) stopShareCleanup() {
	if m.shareCleanupCancel == nil {
		return
	}
	m.shareCleanupCancel()
	<-m.shareCleanupDone
	m.shareCleanupCancel = nil
	m.shareCleanupDone = nil
}

func hasMaxMindSources(available []source.ID) bool {
	for _, id := range available {
		if id == source.MaxMindGeoLite2City || id == source.MaxMindGeoLite2ASN {
			return true
		}
	}
	return false
}
