CREATE TRIGGER set_written_expires_at_trigger
    BEFORE INSERT ON written_consultations
    FOR EACH ROW EXECUTE PROCEDURE set_expires_at_written();
