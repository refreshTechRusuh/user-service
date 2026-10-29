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

-- member_nik: daftar NIK milik user login (keluarga), dipilih saat beli tiket.
-- Field {nik,nama,tgl_lahir,nama_ibu} dicocokkan ke Dukcapil saat simpan.
-- UNIQUE(user_id, nik) = 1 NIK tak boleh dobel untuk user yang sama.
CREATE TABLE IF NOT EXISTS member_nik (
    id         UUID PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    nik        CHAR(16) NOT NULL,
    nama       TEXT NOT NULL,
    tgl_lahir  DATE NOT NULL,
    nama_ibu   TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT member_nik_user_nik_unique UNIQUE (user_id, nik)
);

CREATE INDEX IF NOT EXISTS idx_member_nik_user ON member_nik(user_id);
