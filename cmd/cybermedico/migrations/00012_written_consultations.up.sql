CREATE TABLE written_consultations (
    consultation_id UUID PRIMARY KEY REFERENCES consultations(id) ON DELETE CASCADE,
    doctor_answer TEXT,
    email_reminder_sent BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS update_written_consultations_updated_at ON written_consultations;
CREATE TRIGGER update_written_consultations_updated_at
    BEFORE UPDATE ON written_consultations
    FOR EACH ROW EXECUTE PROCEDURE update_updated_at_column();
    