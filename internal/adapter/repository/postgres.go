package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qkhanh39/go-job-processing/internal/entities"
	"github.com/qkhanh39/go-job-processing/internal/service/job"
)

type jobRepository struct {
	db *pgxpool.Pool
}

func New(
	db *pgxpool.Pool,
) job.JobRepository {
	return &jobRepository{
		db: db,
	}
}

func (r *jobRepository) Create(ctx context.Context, entity *entities.Job) (*entities.Job, error) {
	query := `
		INSERT INTO jobs(
			id,
			type,
			payload,
			status,
			max_attempts
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING
			id,
			type,
			payload,
			status,
			attempts,
			max_attempts,
			result,
			last_error,
			run_at,
			created_at,
			updated_at
	`

	created := &entities.Job{}
	err := r.db.QueryRow(
		ctx,
		query,
		entity.ID,
		entity.Type,
		entity.Payload,
		entity.Status,
		entity.MaxAttempts,
	).Scan(
		&created.ID,
		&created.Type,
		&created.Payload,
		&created.Status,
		&created.Attempts,
		&created.MaxAttempts,
		&created.Result,
		&created.LastError,
		&created.RunAt,
		&created.CreatedAt,
		&created.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return created, nil
}

func (r *jobRepository) ClaimNext(ctx context.Context) (*entities.Job, error) {
	query := `
		WITH next_job AS (
			SELECT id
			FROM jobs
			WHERE status = 'PENDING'
				AND run_at <= NOW()
			ORDER BY run_at, created_at
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE jobs AS j
		SET
			status = 'PROCESSING',
			updated_at = NOW()
		FROM next_job
		WHERE j.id = next_job.id
		RETURNING
			j.id,
			j.type,
			j.payload,
			j.status,
			j.attempts,
			j.max_attempts,
			j.result,
			j.last_error,
			j.run_at,
			j.created_at,
			j.updated_at
	`

	nextJob := &entities.Job{}
	err := r.db.QueryRow(ctx, query).Scan(
		&nextJob.ID,
		&nextJob.Type,
		&nextJob.Payload,
		&nextJob.Status,
		&nextJob.Attempts,
		&nextJob.MaxAttempts,
		&nextJob.Result,
		&nextJob.LastError,
		&nextJob.RunAt,
		&nextJob.CreatedAt,
		&nextJob.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
	}

	return nextJob, nil
}

func (r *jobRepository) StartAttempt(ctx context.Context, id string) (*entities.Job, error) {
	query := `
		UPDATE jobs
		SET 
			attempts = attempts + 1,
			updated_at = NOW()
		WHERE id = $1
			AND status = 'PROCESSING'
		RETURNING
			id, 
			type,
			payload,
			status,
			attempts,
			max_attempts,
			result,
			last_error,
			run_at,
			created_at,
			updated_at
	`

	updated := &entities.Job{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&updated.ID,
		&updated.Type,
		&updated.Payload,
		&updated.Status,
		&updated.Attempts,
		&updated.MaxAttempts,
		&updated.Result,
		&updated.LastError,
		&updated.RunAt,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("start job attempt: %w", err)
	}

	return updated, nil
}

func (r *jobRepository) Complete(ctx context.Context, id string, result json.RawMessage) error {
	query := `
		UPDATE jobs
		SET 
			result = $1,
			status = 'COMPLETED',
			last_error = NULL,
			updated_at = NOW()
		WHERE 
			id = $2
			AND status = 'PROCESSING'
	`

	tag, err := r.db.Exec(ctx, query, result, id)
	if err != nil {
		return fmt.Errorf("complete job: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("complete job: job %s not found or not processing", id)
	}

	return nil
}

func (r *jobRepository) ScheduleRetry(ctx context.Context, id string, runAt time.Time, lastError string) error {
	query := `
		UPDATE jobs
		SET
			run_at = $2,
			last_error = $3,
			status = 'PENDING',
			updated_at = NOW()
		WHERE
			id = $1
			AND status = 'PROCESSING'
	`

	tag, err := r.db.Exec(ctx, query, id, runAt, lastError)
	if err != nil {
		return fmt.Errorf("schedule retry job: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("schedule retry job: job %s not found or not processing", id)
	}

	return nil
}

func (r *jobRepository) MarkFailed(ctx context.Context, id string, lastError string) error {
	query := `
			UPDATE jobs
			SET
				status = 'FAILED',
				last_error = $2,
				updated_at = NOW()
			WHERE
				id = $1
				AND status = 'PROCESSING'
		`

	tag, err := r.db.Exec(ctx, query, id, lastError)
	if err != nil {
		return fmt.Errorf("mark failed job: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark failed job: job %s not found or not processing", id)
	}

	return nil
}
