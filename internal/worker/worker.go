package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/qkhanh39/go-job-processing/internal/entities"
)

type JobService interface {
	ClaimNext(ctx context.Context) (*entities.Job, error)
	Execute(ctx context.Context, job *entities.Job) error
}

type Worker struct {
	jobService   JobService
	wakeUp       <-chan struct{}
	pollInterval time.Duration
}

func New(
	jobService JobService,
	wakeUp <-chan struct{},
	pollInterval time.Duration,
) *Worker {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	return &Worker{
		jobService:   jobService,
		wakeUp:       wakeUp,
		pollInterval: pollInterval,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	slog.Info(
		"worker started",
		"poll_interval", w.pollInterval,
	)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.wakeUp:
		case <-ticker.C:
		}

		if err := w.processJobs(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			slog.Error(
				"worker failed to process job",
				"err", err,
			)

			if err := w.wait(ctx, w.pollInterval); err != nil {
				return err
			}
		}
	}
}

func (w *Worker) processJobs(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		job, err := w.jobService.ClaimNext(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			return err
		}

		if job == nil {
			return nil
		}

		if err := w.jobService.Execute(ctx, job); err != nil {
			slog.Error(
				"failed to executed job",
				"job_id", job.ID,
				"job_type", job.Type,
				"error", err,
			)
		}
	}
}

func (w *Worker) wait(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
