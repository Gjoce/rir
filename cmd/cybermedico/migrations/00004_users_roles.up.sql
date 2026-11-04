CREATE TABLE users_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, role_id)
);

DROP TRIGGER IF EXISTS update_users_roles_updated_at ON users_roles;
CREATE TRIGGER update_users_roles_updated_at
    BEFORE UPDATE ON users_roles
    FOR EACH ROW EXECUTE PROCEDURE update_updated_at_column();
    