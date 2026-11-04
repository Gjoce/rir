CREATE TABLE doctor_financials (
    doctor_id UUID PRIMARY KEY NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bank_account TEXT NOT NULL,
    provision INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS update_doctor_financials_updated_at ON doctor_financials;
CREATE TRIGGER update_doctor_financials_updated_at
    BEFORE UPDATE ON doctor_financials
    FOR EACH ROW EXECUTE PROCEDURE update_updated_at_column();
