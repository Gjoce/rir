CREATE TYPE consultation_type AS ENUM ('video', 'written');
CREATE TYPE consultation_status AS ENUM ('scheduled', 'pending', 'finished', 'not_attended');
CREATE TYPE consultation_payment_status AS ENUM ('pending', 'paid', 'failed');
CREATE TABLE consultations (
    id UUID PRIMARY KEY,
    patient_id UUID REFERENCES users(id) ON DELETE CASCADE,
    doctor_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    consultation_type consultation_type NOT NULL,
    consultation_status consultation_status NOT NULL,
    consultation_payment_status consultation_payment_status NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    price_mkd DECIMAL(10,2) NOT NULL,
    urgent BOOLEAN NOT NULL DEFAULT FALSE,
    need_translator BOOLEAN NOT NULL DEFAULT FALSE,
    terms_accepted BOOLEAN NOT NULL DEFAULT FALSE,
    email_sent_patient BOOLEAN NOT NULL DEFAULT FALSE,
    signature_doctor_url TEXT,
    patient_name TEXT NOT NULL,
    patient_surname TEXT NOT NULL,
    patient_age INTEGER NOT NULL,
    patient_phone TEXT NOT NULL,
    patient_email TEXT NOT NULL,
    patient_place_of_living TEXT,
    how_did_you_find_out TEXT,
    medical_problem TEXT NOT NULL,
    patient_diagnosis TEXT,
    allergies TEXT,
    medication TEXT,
    attachments TEXT[],
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS update_consultations_updated_at ON consultations;
CREATE TRIGGER update_consultations_updated_at
    BEFORE UPDATE ON consultations
    FOR EACH ROW EXECUTE PROCEDURE update_updated_at_column();
