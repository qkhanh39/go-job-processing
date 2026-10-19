package job

import (
	"context"
	"encoding/json"
	"time"

	"github.com/qkhanh39/go-job-processing/internal/entities"
	"github.com/qkhanh39/go-job-processing/internal/service/dto/job"
)

type JobRepository interface {
	Create(ctx context.Context, entity *entities.Job) (*entities.Job, error)
	ClaimNext(ctx context.Context) (*entities.Job, error)
	StartAttempt(ctx context.Context, id string) (*entities.Job, error)
	Complete(ctx context.Context, id string, result json.RawMessage) error
	ScheduleRetry(ctx context.Context, id string, runAt time.Time, lastError string) error
	MarkFailed(ctx context.Context, id string, lastError string) error
}

type JobService interface {
	Create(ctx context.Context, job *job.CreateJobInput) (*entities.Job, error)
	Execute(ctx context.Context, job *entities.Job) error
	ClaimNext(ctx context.Context) (*entities.Job, error)
}
