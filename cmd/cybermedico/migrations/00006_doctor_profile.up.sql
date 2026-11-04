CREATE TABLE doctor_profile (
    id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    specialization TEXT NOT NULL,
    subspecialization TEXT,
    display_name TEXT NOT NULL,
    photo_url TEXT NOT NULL,
    bio TEXT NOT NULL,
    workplace TEXT NOT NULL, 
    available BOOLEAN NOT NULL DEFAULT TRUE,
    consultation_price_video NUMERIC(10,2) NOT NULL,
    consultation_price_written NUMERIC(10,2) NOT NULL,
    position INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS update_doctor_profile_updated_at ON doctor_profile;
CREATE TRIGGER update_doctor_profile_updated_at
    BEFORE UPDATE ON doctor_profile
    FOR EACH ROW EXECUTE PROCEDURE update_updated_at_column();
    