CREATE TABLE ai_analysis_runs (
    id BIGSERIAL PRIMARY KEY,

    project_id BIGINT NOT NULL
        REFERENCES projects(id) ON DELETE CASCADE,

    -- The authenticated user who requested this analysis.
    requested_by_user_id BIGINT NOT NULL
        REFERENCES users(id) ON DELETE RESTRICT,

    -- The run moves through this lifecycle.
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),

    -- Versioned, sanitized project evidence sent to the agent.
    context_schema_version SMALLINT NOT NULL DEFAULT 1,
    context_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb
        CHECK (jsonb_typeof(context_snapshot) = 'object'),

    -- Structured result returned by the agent after validation.
    result JSONB
        CHECK (result IS NULL OR jsonb_typeof(result) = 'object'),

    -- Provider/model names are useful for debugging and cost tracking.
    -- They never contain API keys.
    provider TEXT,
    model TEXT,

    attempt_count INTEGER NOT NULL DEFAULT 0
        CHECK (attempt_count >= 0),

    error_message TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Active runs have no completion time; terminal runs always do.
    CHECK (
        (status IN ('queued', 'running') AND completed_at IS NULL)
        OR (
            status IN ('succeeded', 'failed', 'cancelled')
            AND completed_at IS NOT NULL
        )
    ),

    -- A run cannot finish before it starts.
    CHECK (
        completed_at IS NULL
        OR started_at IS NULL
        OR completed_at >= started_at
    )
);

-- Shows a project's newest analysis runs efficiently.
CREATE INDEX ai_analysis_runs_project_created_idx
    ON ai_analysis_runs(project_id, created_at DESC, id DESC);

-- Supports a future worker looking for queued work.
CREATE INDEX ai_analysis_runs_pending_idx
    ON ai_analysis_runs(created_at ASC, id ASC)
    WHERE status IN ('queued', 'running');
