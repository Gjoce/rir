CREATE TABLE doctor_language (
    doctor_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    language_id TEXT NOT NULL REFERENCES languages(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (doctor_id, language_id)
);

DROP TRIGGER IF EXISTS update_doctor_language_updated_at ON doctor_language;
CREATE TRIGGER update_doctor_language_updated_at
    BEFORE UPDATE ON doctor_language
    FOR EACH ROW EXECUTE PROCEDURE update_updated_at_column();
