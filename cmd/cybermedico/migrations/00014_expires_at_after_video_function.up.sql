CREATE OR REPLACE FUNCTION set_expires_at_after_video()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.scheduled_at IS NOT NULL THEN
        NEW.expires_at_after_video := NEW.scheduled_at + INTERVAL '24 hours';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE 'plpgsql';