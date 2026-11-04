CREATE OR REPLACE FUNCTION set_requires_response_by_schedule()
RETURNS TRIGGER AS $$
DECLARE
    is_urgent BOOLEAN;
BEGIN
    SELECT urgent INTO is_urgent FROM consultations WHERE id = NEW.consultation_id;
    IF is_urgent THEN
        NEW.requires_response_by_schedule := NEW.created_at + INTERVAL '8 hours';
    ELSE
        NEW.requires_response_by_schedule := NEW.created_at + INTERVAL '12 hours';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
