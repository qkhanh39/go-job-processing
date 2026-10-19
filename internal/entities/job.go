package entities

import (
	"encoding/json"
	"time"
)

type Job struct {
	ID          string
	Type        JobType
	Payload     json.RawMessage
	Status      Status
	Attempts    int
	MaxAttempts int
	Result      json.RawMessage
	LastError   *string
	RunAt       time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}