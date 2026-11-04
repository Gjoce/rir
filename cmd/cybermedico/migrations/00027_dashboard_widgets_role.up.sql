ALTER TABLE dashboard_widgets DROP COLUMN user_id;

ALTER TABLE dashboard_widgets
    ADD COLUMN role TEXT NOT NULL DEFAULT 'doctor' REFERENCES roles(id) ON DELETE CASCADE;

ALTER TABLE dashboard_widgets ALTER COLUMN role DROP DEFAULT;
