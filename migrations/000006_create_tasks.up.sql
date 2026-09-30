
CREATE TABLE tasks (
    id BIGSERIAL PRIMARY KEY,

    project_id BIGINT NOT NULL
        REFERENCES projects(id) ON DELETE CASCADE,

    created_by_user_id BIGINT NOT NULL
        REFERENCES users(id) ON DELETE RESTRICT,

    assignee_id BIGINT
        REFERENCES users(id) ON DELETE SET NULL,

    title VARCHAR(150) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    acceptance_criteria TEXT NOT NULL DEFAULT '',

    -- Built automatically from task content for full-text search.
    search_vector TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('english', title), 'A') ||
        setweight(to_tsvector('english', description), 'B') ||
        setweight(to_tsvector('english', acceptance_criteria), 'C')
    ) STORED,

    status TEXT NOT NULL DEFAULT 'todo'
        CHECK (status IN ('todo', 'in_progress', 'blocked', 'done')),

    priority TEXT NOT NULL DEFAULT 'medium'
        CHECK (priority IN ('low', 'medium', 'high', 'urgent')),

    due_date DATE,
    estimate_minutes INTEGER,
    blocked_reason TEXT,

    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    -- Increments after every successful update for optimistic concurrency.
    version INTEGER NOT NULL DEFAULT 1,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Validate task content.
    CHECK (char_length(BTRIM(title)) > 0),
    CHECK (char_length(description) <= 5000),
    CHECK (char_length(acceptance_criteria) <= 3000),
    CHECK (estimate_minutes IS NULL OR estimate_minutes > 0),

    -- Blocked tasks must have a reason.
    -- Other tasks must not have a blocked reason.
    CHECK (
        (
            status = 'blocked'
            AND char_length(BTRIM(COALESCE(blocked_reason, ''))) > 0
        )
        OR (
            status <> 'blocked'
            AND blocked_reason IS NULL
        )
    ),

    -- Completed tasks must have both timestamps.
    -- Other tasks must not have a completion timestamp.
    CHECK (
        (
            status = 'done'
            AND started_at IS NOT NULL
            AND completed_at IS NOT NULL
        )
        OR (
            status <> 'done'
            AND completed_at IS NULL
        )
    ),

    -- Completion cannot occur before the task starts.
    CHECK (
        completed_at IS NULL
        OR completed_at >= started_at
    )
);


-- Index for retrieving tasks within a project,
-- ordered by their most recent updates.
CREATE INDEX tasks_project_id_updated_at_idx
    ON tasks(project_id, updated_at DESC);


-- Index for filtering project tasks by status.
CREATE INDEX tasks_project_id_status_idx
    ON tasks(project_id, status);


-- Partial index for finding assigned tasks by due date.
CREATE INDEX tasks_assignee_id_due_date_idx
    ON tasks(assignee_id, due_date)
    WHERE assignee_id IS NOT NULL;


-- GIN index for efficient full-text search.
CREATE INDEX tasks_search_vector_idx
    ON tasks USING GIN(search_vector);
