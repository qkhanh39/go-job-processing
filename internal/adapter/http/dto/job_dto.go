package dto

import (
	"encoding/json"
	"time"

	"github.com/qkhanh39/go-job-processing/internal/entities"
	"github.com/qkhanh39/go-job-processing/internal/service/dto/job"
)

type CreateJobRequest struct {
	Type    entities.JobType `json:"type"`
	Payload json.RawMessage  `json:"payload"`
}

func (r CreateJobRequest) ToInput() *job.CreateJobInput {
	return &job.CreateJobInput{
		Type:    r.Type,
		Payload: r.Payload,
	}
}

type CreateJobResponse struct {
	ID          string           `json:"id"`
	Type        entities.JobType `json:"type"`
	Payload     json.RawMessage  `json:"payload"`
	Status      entities.Status  `json:"status"`
	Attempts    int              `json:"attempts"`
	MaxAttempts int              `json:"max_attempts"`
	RunAt       time.Time        `json:"run_at"`
	CreatedAt   time.Time        `json:"created_at"`
}

func ToCreateJobResponse(e *entities.Job) CreateJobResponse {
	return CreateJobResponse{
		ID:          e.ID,
		Type:        e.Type,
		Payload:     e.Payload,
		Status:      e.Status,
		Attempts:    e.Attempts,
		MaxAttempts: e.MaxAttempts,
		RunAt:       e.RunAt,
		CreatedAt:   e.CreatedAt,
	}
}
