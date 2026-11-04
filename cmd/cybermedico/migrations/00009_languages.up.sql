CREATE TABLE languages (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS update_languages_updated_at ON languages;
CREATE TRIGGER update_languages_updated_at
    BEFORE UPDATE ON languages
    FOR EACH ROW EXECUTE PROCEDURE update_updated_at_column();
    