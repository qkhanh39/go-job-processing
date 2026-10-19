package job

import (
	"encoding/json"

	"github.com/qkhanh39/go-job-processing/internal/entities"
)

type CreateJobInput struct {
	Type    entities.JobType
	Payload json.RawMessage
}

func (r CreateJobInput) ToEntity() *entities.Job {
	return &entities.Job{
		Type:    r.Type,
		Payload: r.Payload,
	}
}

type SendEmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type SendEmailResult struct {
	Sent bool `json:"sent"`
}
