package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qkhanh39/go-job-processing/internal/entities"
)

// fakeJobService is a JobService double for pool tests. ClaimNext is only ever
// called by the single dispatcher goroutine; Execute may be called by many
// executors concurrently.
type fakeJobService struct {
	mu       sync.Mutex
	pending  []*entities.Job
	claimErr error

	claimAttempts atomic.Int64
	claimed       atomic.Int64
	executed      atomic.Int64

	executeFn func(ctx context.Context, job *entities.Job) error
}

func newJobs(n int) []*entities.Job {
	jobs := make([]*entities.Job, n)
	for i := range jobs {
		jobs[i] = &entities.Job{
			ID:   fmt.Sprintf("job-%d", i),
			Type: entities.JobTypeSendEmail,
		}
	}

	return jobs
}

func (f *fakeJobService) ClaimNext(ctx context.Context) (*entities.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.claimAttempts.Add(1)

	if f.claimErr != nil {
		return nil, f.claimErr
	}

	if len(f.pending) == 0 {
		return nil, nil
	}

	job := f.pending[0]
	f.pending = f.pending[1:]
	f.claimed.Add(1)

	return job, nil
}

func (f *fakeJobService) Execute(ctx context.Context, job *entities.Job) error {
	if f.executeFn != nil {
		if err := f.executeFn(ctx, job); err != nil {
			return err
		}
	}

	f.executed.Add(1)

	return nil
}

// waitFor polls cond until it returns true or the timeout elapses.
func waitFor(cond func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if cond() {
			return true
		}

		time.Sleep(5 * time.Millisecond)
	}

	return cond()
}

// startPool primes the wake-up channel so the dispatcher claims immediately,
// then runs the pool in a goroutine and returns its error channel. A short
// poll interval keeps claim-error retries fast in tests.
func startPool(t *testing.T, svc JobService, workers, queueSize int, shutdownTimeout time.Duration) (chan error, context.CancelFunc) {
	t.Helper()

	wakeUp := make(chan struct{}, 1)
	wakeUp <- struct{}{}

	ctx, cancel := context.WithCancel(context.Background())

	pool := New(svc, wakeUp, 10*time.Millisecond, workers, queueSize, shutdownTimeout)

	runErr := make(chan error, 1)
	go func() {
		runErr <- pool.Run(ctx)
	}()

	return runErr, cancel
}

// stopPool cancels the pool and waits for Run to return context.Canceled.
func stopPool(t *testing.T, runErr chan error, cancel context.CancelFunc) {
	t.Helper()

	cancel()

	select {
	case err := <-runErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pool did not stop after cancel")
	}
}

func TestPoolExecutesJobsConcurrently(t *testing.T) {
	const (
		workers   = 4
		queueSize = 10
		totalJobs = 8
	)

	var inFlight atomic.Int64
	var maxInFlight atomic.Int64
	var releaseOnce sync.Once
	release := make(chan struct{})

	svc := &fakeJobService{
		pending: newJobs(totalJobs),
		executeFn: func(ctx context.Context, job *entities.Job) error {
			cur := inFlight.Add(1)

			for {
				prev := maxInFlight.Load()
				if cur <= prev || maxInFlight.CompareAndSwap(prev, cur) {
					break
				}
			}

			if cur == workers {
				releaseOnce.Do(func() { close(release) })
			}

			<-release
			inFlight.Add(-1)

			return nil
		},
	}

	runErr, cancel := startPool(t, svc, workers, queueSize, time.Second)

	if !waitFor(func() bool { return svc.executed.Load() == int64(totalJobs) }, 2*time.Second) {
		t.Fatalf("expected %d jobs executed, got %d", totalJobs, svc.executed.Load())
	}

	stopPool(t, runErr, cancel)

	if mf := maxInFlight.Load(); mf < 2 || mf > int64(workers) {
		t.Fatalf("expected max in-flight in [2, %d], got %d", workers, mf)
	}
}

func TestPoolRespectsQueueCapacity(t *testing.T) {
	const (
		workers   = 2
		queueSize = 2
		totalJobs = 10
	)

	block := make(chan struct{})
	var blockOnce sync.Once
	releaseBlock := func() { blockOnce.Do(func() { close(block) }) }
	defer releaseBlock()

	svc := &fakeJobService{
		pending: newJobs(totalJobs),
		executeFn: func(ctx context.Context, job *entities.Job) error {
			<-block
			return nil
		},
	}

	runErr, cancel := startPool(t, svc, workers, queueSize, 100*time.Millisecond)

	// Let the dispatcher claim until it saturates the queue and blocks.
	if !waitFor(func() bool { return svc.claimed.Load() >= int64(queueSize+workers+1) }, 2*time.Second) {
		t.Fatalf("dispatcher did not fill the queue, claimed=%d", svc.claimed.Load())
	}

	// The dispatcher must stop claiming once the queue is saturated.
	claimed := svc.claimed.Load()
	if claimed > int64(queueSize+workers+1) {
		t.Fatalf("dispatcher over-claimed: got %d, want <= %d", claimed, queueSize+workers+1)
	}

	if claimed >= int64(totalJobs) {
		t.Fatalf("dispatcher claimed all %d jobs despite a bounded queue", totalJobs)
	}

	stopPool(t, runErr, cancel)
}

func TestPoolGracefulShutdown(t *testing.T) {
	const (
		workers   = 1
		queueSize = 10
		totalJobs = 5
	)

	var inFlight atomic.Int64
	release := make(chan struct{})

	svc := &fakeJobService{
		pending: newJobs(totalJobs),
		executeFn: func(ctx context.Context, job *entities.Job) error {
			if job.ID == "job-0" {
				inFlight.Add(1)
				<-release
				inFlight.Add(-1)
			}

			return nil
		},
	}

	runErr, cancel := startPool(t, svc, workers, queueSize, time.Second)

	// Wait until all jobs are claimed and the first one is in-flight.
	if !waitFor(func() bool {
		return svc.claimed.Load() == int64(totalJobs) && inFlight.Load() == 1
	}, 2*time.Second) {
		t.Fatalf("expected all %d jobs claimed with one in-flight, claimed=%d inFlight=%d",
			totalJobs, svc.claimed.Load(), inFlight.Load())
	}

	cancel()

	// Let the in-flight job finish so the executor can drain the queued jobs.
	close(release)

	stopPool(t, runErr, cancel)

	if svc.executed.Load() != int64(totalJobs) {
		t.Fatalf("expected all %d claimed jobs to execute, got %d", totalJobs, svc.executed.Load())
	}
}

func TestPoolRecoversFromClaimErrors(t *testing.T) {
	const (
		workers   = 2
		queueSize = 4
		totalJobs = 3
	)

	svc := &fakeJobService{
		pending:  newJobs(totalJobs),
		claimErr: errors.New("transient claim failure"),
	}

	runErr, cancel := startPool(t, svc, workers, queueSize, time.Second)

	// The pool must keep attempting to claim despite the error.
	if !waitFor(func() bool { return svc.claimAttempts.Load() > 0 }, 2*time.Second) {
		t.Fatal("pool did not attempt to claim")
	}

	// Clear the transient error so the pool can process the jobs.
	svc.mu.Lock()
	svc.claimErr = nil
	svc.mu.Unlock()

	if !waitFor(func() bool { return svc.executed.Load() == int64(totalJobs) }, 2*time.Second) {
		t.Fatalf("expected %d jobs executed, got %d", totalJobs, svc.executed.Load())
	}

	stopPool(t, runErr, cancel)
}
