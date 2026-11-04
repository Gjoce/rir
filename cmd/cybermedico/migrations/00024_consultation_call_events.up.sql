CREATE TYPE call_event_kind AS ENUM (
    'doctor_joined',
    'doctor_left',
    'patient_joined',
    'patient_left',
    'ice_connected',
    'ice_relay_used',
    'ice_failed',
    'call_ended'
);

CREATE TABLE consultation_call_events (
    id BIGSERIAL PRIMARY KEY,
    consultation_id UUID NOT NULL REFERENCES consultations(id) ON DELETE CASCADE,
    kind call_event_kind NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    detail JSONB
);

CREATE INDEX idx_call_events_consultation ON consultation_call_events(consultation_id, occurred_at);
