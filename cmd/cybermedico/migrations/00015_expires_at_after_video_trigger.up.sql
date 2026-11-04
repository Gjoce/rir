CREATE TRIGGER set_video_expires_at_trigger
    BEFORE INSERT OR UPDATE OF scheduled_at ON video_consultations
    FOR EACH ROW EXECUTE PROCEDURE set_expires_at_after_video();