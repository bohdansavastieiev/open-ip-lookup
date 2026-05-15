package update

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
)

type sourceRefreshJob struct {
	index      int
	id         source.ID
	definition source.Definition
	state      sourceState
}

type sourceRefreshResult struct {
	index      int
	id         source.ID
	definition source.Definition
	update     sourceUpdateResult
	duration   time.Duration
	err        error
}

func (u *Updater) refreshSources(
	ctx context.Context,
	sourceIDs []source.ID,
	sources map[source.ID]sourceState,
	retryDelays []time.Duration,
) ([]sourceRefreshResult, error) {
	jobs := make([]sourceRefreshJob, len(sourceIDs))
	for i, id := range sourceIDs {
		jobs[i] = sourceRefreshJob{
			index:      i,
			id:         id,
			definition: source.DefinitionFor(id),
			state:      sources[id],
		}
	}

	results := make([]sourceRefreshResult, len(sourceIDs))
	if len(jobs) == 0 {
		return results, nil
	}

	jobCh := make(chan sourceRefreshJob)
	resultCh := make(chan sourceRefreshResult, len(jobs))
	workerCount := min(sourceWorkerLimit, len(jobs))
	var wg sync.WaitGroup
	for range workerCount {
		wg.Go(func() {
			for job := range jobCh {
				resultCh <- u.refreshSource(ctx, job, retryDelays)
			}
		})
	}

	go func() {
		defer close(jobCh)
		for _, job := range jobs {
			select {
			case jobCh <- job:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var err error
	for result := range resultCh {
		results[result.index] = result
		if err == nil && result.err != nil {
			err = result.err
		}
	}
	if err != nil {
		cleanupRefreshTemps(results)
		return nil, err
	}
	if ctx.Err() != nil {
		cleanupRefreshTemps(results)
		return nil, ctx.Err()
	}
	return results, nil
}

func (u *Updater) refreshSource(
	ctx context.Context,
	job sourceRefreshJob,
	retryDelays []time.Duration,
) (result sourceRefreshResult) {
	startedAt := time.Now()
	u.logger.Info("source refresh started", slog.String("source", string(job.id)))
	defer func() {
		result.duration = time.Since(startedAt)
		logSourceRefreshResult(u.logger, result)
	}()

	result = sourceRefreshResult{
		index:      job.index,
		id:         job.id,
		definition: job.definition,
	}
	update, err := u.refreshWithRetries(ctx, job.definition, job.state, retryDelays)
	result.update = update
	result.err = err
	return result
}

func logSourceRefreshResult(logger *slog.Logger, result sourceRefreshResult) {
	if result.err != nil {
		logger.Info(
			"source artifact prepare failed",
			slog.String("source", string(result.id)),
			slog.Any("err", result.err),
			slog.Duration("duration", result.duration),
		)
		return
	}
	if !result.update.success {
		logger.Info(
			"source artifact prepare failed",
			slog.String("source", string(result.id)),
			slog.Bool("retryable", result.update.retryable),
			slog.Duration("duration", result.duration),
		)
		return
	}

	if !result.update.changed {
		logger.Info(
			"source refresh checked",
			slog.String("source", string(result.id)),
			slog.String("status", "not_modified"),
			slog.Duration("duration", result.duration),
		)
		return
	}

	logger.Info(
		"source artifact prepared",
		slog.String("source", string(result.id)),
		slog.String("status", "downloaded"),
		slog.Duration("duration", result.duration),
	)
}

func (u *Updater) refreshWithRetries(
	ctx context.Context,
	definition source.Definition,
	savedState sourceState,
	retryDelays []time.Duration,
) (sourceUpdateResult, error) {
	current := savedState
	for attempt := range len(retryDelays) + 1 {
		result, err := refreshHTTPSource(
			ctx,
			u.httpClient,
			definition,
			current,
			updateDirPath(u.cfg.DataDir),
		)
		if err != nil {
			return sourceUpdateResult{}, err
		}
		if result.success || !result.shortRetryable || attempt == len(retryDelays) {
			return result, nil
		}

		u.logger.Info(
			"source refresh retry scheduled",
			slog.String("source", string(definition.ID)),
			slog.Int("attempt", attempt+1),
			slog.Duration("delay", retryDelays[attempt]),
			slog.Bool("retryable", result.retryable),
		)
		current = result.state
		if err := sleepOrCancel(ctx, retryDelays[attempt]); err != nil {
			return sourceUpdateResult{}, err
		}
	}

	return sourceUpdateResult{}, errors.New("unreachable source refresh retry state")
}

func cleanupRefreshTemps(results []sourceRefreshResult) {
	for _, result := range results {
		if result.update.tempPath != "" {
			_ = os.RemoveAll(result.update.tempPath)
		}
	}
}

func sleepOrCancel(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer stopTimer(timer)

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
