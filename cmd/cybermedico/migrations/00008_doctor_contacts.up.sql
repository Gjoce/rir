CREATE TABLE doctor_contacts (
    doctor_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    phone_number TEXT NOT NULL,
    meeting_link TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS update_doctor_contacts_updated_at ON doctor_contacts;
CREATE TRIGGER update_doctor_contacts_updated_at
    BEFORE UPDATE ON doctor_contacts
    FOR EACH ROW EXECUTE PROCEDURE update_updated_at_column();
    