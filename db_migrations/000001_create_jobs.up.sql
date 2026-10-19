CREATE TABLE jobs (
    id UUID PRIMARY KEY,
    type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',

    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,

    result JSONB,
    last_error TEXT,

    run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT jobs_status_check
        CHECK (
            status IN (
                'PENDING',
                'PROCESSING',
                'COMPLETED',
                'FAILED'
            )
        ),

    CONSTRAINT jobs_attempts_check
        CHECK (attempts >= 0 AND max_attempts > 0)
);

CREATE INDEX idx_jobs_pending_run_at
    ON jobs (run_at, created_at)
    WHERE status = 'PENDING';