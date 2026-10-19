package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qkhanh39/go-job-processing/internal/entities"
	jobDto "github.com/qkhanh39/go-job-processing/internal/service/dto/job"
)

type sendEmailHandler struct {
}

func NewSendEmailJobHandler() JobHandler {
	return &sendEmailHandler{}
}

func (h *sendEmailHandler) ExecuteJob(ctx context.Context, job *entities.Job) (json.RawMessage, error) {
	var payload jobDto.SendEmailPayload

	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return nil, fmt.Errorf("decode email payload: %w", err)
	}

	result, err := json.Marshal(map[string]any{
		"sent": true,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal email result: %w", err)
	}

	return result, nil
}
