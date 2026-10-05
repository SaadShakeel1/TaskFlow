CREATE TABLE activity_logs (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL
        REFERENCES projects(id) ON DELETE CASCADE,
    user_id BIGINT
        REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL
        CHECK (BTRIM(action)<> ''),
    entity_type TEXT NOT NULL
        CHECK (BTRIM(entity_type)<> ''),
    entity_id BIGINT,
    metadata JSONB NOT NULL DEFAULT '{}' :: jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX activity_logs_project_created_idx
    ON activity_logs(project_id,created_at DESC, id DESC);
