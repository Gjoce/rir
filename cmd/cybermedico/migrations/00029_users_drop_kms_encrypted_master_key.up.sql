-- Private keys are encrypted with the app's signing key (SIGNING_KEY) instead of
-- a KMS-wrapped key per doctor.
ALTER TABLE users
    DROP COLUMN kms_encrypted_master_key;
