CREATE TYPE preferred_time_slot AS ENUM ('morning', 'afternoon', 'evening');

CREATE TABLE video_consultations (
    consultation_id UUID PRIMARY KEY REFERENCES consultations(id) ON DELETE CASCADE,
    preferred_time preferred_time_slot NOT NULL,
    scheduled_at TIMESTAMPTZ,
    requires_response_by_schedule TIMESTAMPTZ,
    expires_at_after_video TIMESTAMPTZ,
    doctor_answer_after_video TEXT,
    reminder_sent_scheduling BOOLEAN NOT NULL DEFAULT FALSE,
    reminder_sent_answer BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS update_video_consultations_updated_at ON video_consultations;
CREATE TRIGGER update_video_consultations_updated_at
    BEFORE UPDATE ON video_consultations
    FOR EACH ROW EXECUTE PROCEDURE update_updated_at_column();
    