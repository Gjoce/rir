CREATE TRIGGER set_requires_response_by_schedule_trigger
    BEFORE INSERT ON video_consultations
    FOR EACH ROW EXECUTE PROCEDURE set_requires_response_by_schedule();
