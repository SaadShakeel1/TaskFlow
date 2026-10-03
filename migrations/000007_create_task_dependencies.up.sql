CREATE TABLE task_dependencies (
    task_id BIGINT NOT NULL
        REFERENCES tasks(id) ON DELETE CASCADE,
    depends_on_task_id BIGINT NOT NULL
        REFERENCES tasks(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (task_id, depends_on_task_id),
    CHECK (task_id <> depends_on_task_id)
);

CREATE INDEX task_dependencies_depends_on_task_id_idx                               
    ON task_dependencies(depends_on_task_id);