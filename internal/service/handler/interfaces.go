package handler

import (
	"context"
	"encoding/json"

	"github.com/qkhanh39/go-job-processing/internal/entities"
)

type JobHandler interface {
	ExecuteJob(ctx context.Context, job *entities.Job) (json.RawMessage, error)
}
