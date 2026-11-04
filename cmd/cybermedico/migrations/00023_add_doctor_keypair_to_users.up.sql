ALTER TABLE users
    ADD COLUMN public_key                BYTEA,
    ADD COLUMN kms_encrypted_master_key  BYTEA,
    ADD COLUMN encrypted_private_key     BYTEA,
    ADD COLUMN key_revoked_at            TIMESTAMPTZ,
    ALTER COLUMN password_hash           DROP NOT NULL;
