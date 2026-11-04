ALTER TABLE users
    ADD COLUMN totp_pending_secret TEXT,
    ADD COLUMN totp_pending_secret_expires_at TIMESTAMPTZ,
    ADD COLUMN totp_secret TEXT,
    ADD COLUMN totp_enrolled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN totp_last_used_step BIGINT;