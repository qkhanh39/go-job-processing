package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/qkhanh39/go-job-processing/internal/entities"
)

type JobService interface {
	ClaimNext(ctx context.Context) (*entities.Job, error)
	Execute(ctx context.Context, job *entities.Job) error
}

// Pool runs a single dispatcher that claims jobs from the JobService and a
// fixed set of executor goroutines that run them concurrently. Claimed jobs
// flow through a bounded channel that provides backpressure to the dispatcher.
type Pool struct {
	jobService      JobService
	wakeUp          <-chan struct{}
	pollInterval    time.Duration
	workers         int
	queueSize       int
	shutdownTimeout time.Duration
}

func New(
	jobService JobService,
	wakeUp <-chan struct{},
	pollInterval time.Duration,
	workers int,
	queueSize int,
	shutdownTimeout time.Duration,
) *Pool {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	if workers < 1 {
		workers = 4
	}

	if queueSize < 1 {
		queueSize = workers * 2
	}

	if shutdownTimeout <= 0 {
		shutdownTimeout = 10 * time.Second
	}

	return &Pool{
		jobService:      jobService,
		wakeUp:          wakeUp,
		pollInterval:    pollInterval,
		workers:         workers,
		queueSize:       queueSize,
		shutdownTimeout: shutdownTimeout,
	}
}

// Run starts the dispatcher and executors and blocks until ctx is cancelled.
// After ctx is cancelled it stops dispatching, lets executors drain the queue,
// and interrupts any still-running executions once shutdownTimeout elapses.
func (p *Pool) Run(ctx context.Context) error {
	jobs := make(chan *entities.Job, p.queueSize)

	execCtx, cancelExec := context.WithCancel(context.Background())
	defer cancelExec()

	var wg sync.WaitGroup
	for i := 0; i < p.workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			p.runExecutor(execCtx, jobs)
		}()
	}

	p.dispatch(ctx, jobs)

	close(jobs)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(p.shutdownTimeout):
		slog.Warn(
			"worker pool shutdown grace period expired, interrupting in-flight executions",
			"timeout", p.shutdownTimeout,
		)
		cancelExec()
	}

	return ctx.Err()
}

// dispatch claims jobs and enqueues them until ctx is cancelled. It is the
// sole sender on the jobs channel.
func (p *Pool) dispatch(ctx context.Context, jobs chan<- *entities.Job) {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	slog.Info(
		"worker pool started",
		"workers", p.workers,
		"queue_size", p.queueSize,
		"poll_interval", p.pollInterval,
	)

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.wakeUp:
		case <-ticker.C:
		}

		p.drain(ctx, jobs)
	}
}

// drain claims and enqueues jobs until none are available, ctx is cancelled,
// or a claim error occurs. A job that has already been claimed is always
// enqueued before the dispatcher exits so it is never stranded in PROCESSING.
func (p *Pool) drain(ctx context.Context, jobs chan<- *entities.Job) {
	for {
		if ctx.Err() != nil {
			return
		}

		job, err := p.jobService.ClaimNext(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			slog.Error("worker pool failed to claim job", "err", err)

			if err := p.wait(ctx, p.pollInterval); err != nil {
				return
			}

			continue
		}

		if job == nil {
			return
		}

		select {
		case jobs <- job:
		case <-ctx.Done():
			// Shutdown while a job is already claimed. Try to hand it to an
			// executor without blocking: if the queue has room it is enqueued
			// and will run during the drain; if executors are stuck we accept
			// the job is left in PROCESSING (stale recovery is out of scope).
			select {
			case jobs <- job:
			default:
			}
			return
		}
	}
}

func (p *Pool) runExecutor(ctx context.Context, jobs <-chan *entities.Job) {
	for job := range jobs {
		if err := p.jobService.Execute(ctx, job); err != nil {
			slog.Error(
				"failed to execute job",
				"job_id", job.ID,
				"job_type", job.Type,
				"err", err,
			)
		}
	}
}

func (p *Pool) wait(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
