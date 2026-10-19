package job

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/qkhanh39/go-job-processing/internal/entities"
	"github.com/qkhanh39/go-job-processing/internal/notification"
	"github.com/qkhanh39/go-job-processing/internal/service/dto/job"
	"github.com/qkhanh39/go-job-processing/internal/service/handler"
)

type jobService struct {
	jobRps      JobRepository
	maxAttempts int
	handlers    map[entities.JobType]handler.JobHandler
	notifier    notification.JobNotifier
}

func New(
	jobRps JobRepository,
	maxAttempts int,
	handlers map[entities.JobType]handler.JobHandler,
	notifier notification.JobNotifier,
) JobService {
	return &jobService{
		jobRps:      jobRps,
		maxAttempts: maxAttempts,
		handlers:    handlers,
		notifier:    notifier,
	}
}

func (s *jobService) Create(ctx context.Context, jobRequest *job.CreateJobInput) (*entities.Job, error) {
	err := s.validateInput(jobRequest)
	if err != nil {
		return nil, err
	}

	job := jobRequest.ToEntity()
	job.ID = uuid.NewString()
	job.Status = entities.StatusPending
	job.MaxAttempts = s.maxAttempts

	job, err = s.jobRps.Create(ctx, job)
	if err != nil {
		return nil, fmt.Errorf("create job %w", err)
	}

	s.notifier.Notify()

	return job, nil
}

func (s *jobService) validateInput(jobInput *job.CreateJobInput) error {
	switch jobInput.Type {
	case entities.JobTypeSendEmail:
		return s.validateEmailInput(jobInput.Payload)
	default:
		return entities.ErrUnsupportedJobType
	}
}

func (s *jobService) validateEmailInput(input json.RawMessage) error {
	var payload job.SendEmailPayload

	if err := json.Unmarshal(input, &payload); err != nil {
		return entities.ErrInvalidPayload
	}

	if payload.To == "" || payload.Subject == "" || payload.Body == "" {
		return entities.ErrInvalidPayload
	}

	return nil
}

func (s *jobService) Execute(ctx context.Context, job *entities.Job) error {
	handler, ok := s.handlers[job.Type]
	if !ok {
		return entities.ErrUnsupportedJobType
	}

	job, err := s.jobRps.StartAttempt(ctx, job.ID)
	if err != nil {
		return err
	}

	result, err := handler.ExecuteJob(ctx, job)
	if err != nil {
		return s.handleFailure(ctx, job, err)
	}

	if err := s.jobRps.Complete(ctx, job.ID, result); err != nil {
		return err
	}

	return nil
}

func (s *jobService) handleFailure(ctx context.Context, job *entities.Job, cause error) error {
	lastError := cause.Error()

	if job.Attempts >= job.MaxAttempts {
		if err := s.jobRps.MarkFailed(ctx, job.ID, lastError); err != nil {
			return err
		}

		return nil
	}

	runAt := time.Now().Add(s.retryDelay(job.Attempts))

	if err := s.jobRps.ScheduleRetry(ctx, job.ID, runAt, lastError); err != nil {
		return err
	}

	return nil
}

func (s *jobService) retryDelay(attempts int) time.Duration {
	const (
		baseDelay = time.Second
		maxDelay  = time.Minute
	)

	if attempts < 1 {
		attempts = 1
	}

	delay := baseDelay

	for i := 1; i < attempts; i++ {
		if delay >= maxDelay/2 {
			return maxDelay
		}

		delay *= 2
	}

	return delay
}

func (s *jobService) ClaimNext(ctx context.Context) (*entities.Job, error) {
	return s.jobRps.ClaimNext(ctx)
}
