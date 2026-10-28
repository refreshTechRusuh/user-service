-- migrations/001_initial_schema.sql
-- user-service: tabel akun (app_user).
-- id = UUID (di-generate aplikasi via google/uuid). email UNIK (login).
-- role = customer | admin. password_hash = bcrypt.

CREATE TABLE IF NOT EXISTS app_user (
    id            UUID PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'customer',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT app_user_role_check CHECK (role IN ('customer','admin'))
);

CREATE INDEX IF NOT EXISTS idx_app_user_email ON app_user(email);
