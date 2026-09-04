ALTER TABLE users
    ADD COLUMN password_hash BYTEA,
    ADD COLUMN email_verified_at TIMESTAMPTZ;

CREATE TABLE email_verification_tokens(
    token_hash BYTEA PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE
        REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (expires_at>created_at)
);

CREATE INDEX email_verification_tokens_expires_at_idx
    ON email_verification_tokens(expires_at);