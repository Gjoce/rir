CREATE OR REPLACE FUNCTION set_expires_at_written()
RETURNS TRIGGER AS $$
BEGIN
    NEW.expires_at := NEW.created_at + INTERVAL '72 hours';
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
