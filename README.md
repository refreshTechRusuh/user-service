# user-service

Layanan **identitas & akun** untuk **Rusuh Booking Tiket**: registrasi, login (JWT),
profil, logout, CRUD member/NIK (divalidasi ke Dukcapil), dan riwayat tiket.

- Porsi: **BE-1 (Identity & Flow)**
- Arsitektur: Hexagonal (Ports & Adapters), `net/http` + `pgx` + Redis
- Port default: **8086**
- Login **opsional** untuk journey beli tiket (pola KAI); akun dipakai untuk simpan
  NIK & riwayat. `role` membedakan `customer` vs `admin`.

## Prasyarat

- Go 1.26+
- PostgreSQL + Redis (infra bersama di repo `tools` via docker-compose)
- dukcapil-mock jalan di `:8085` (untuk endpoint member — validasi NIK)

## Menjalankan

```bash
# 1. Infra bersama (dari repo tools, sekali)
cd tools && docker compose up -d postgres redis

# 2. Jalankan service (dari repo ini)
make run        # = go run ./cmd  (pakai env default → localhost)
```

Service listen di `:8086`. Berhenti dengan `Ctrl+C`.

## Endpoint

Auth (publik kecuali disebut) — **tanpa versi** di path (titik tetap lintas-service):

| Method | Path             | Keterangan                                 |
|--------|------------------|--------------------------------------------|
| POST   | `/auth/register` | Daftar akun customer (201)                 |
| POST   | `/auth/login`    | Login → authToken + refreshToken (JWT)     |
| GET    | `/auth/me`       | Profil dari token (butuh authToken)        |
| POST   | `/auth/logout`   | Revoke refresh token (idempoten)           |

Member/NIK & Riwayat (butuh authToken) — pola `/api/user/v1/...`:

| Method | Path                                   | Keterangan                          |
|--------|----------------------------------------|-------------------------------------|
| POST   | `/api/user/v1/member/create`           | Tambah NIK (dicocokkan ke Dukcapil) |
| GET    | `/api/user/v1/member/list`             | Daftar member milik user            |
| PUT    | `/api/user/v1/member/update/{id}`      | Ubah member (re-validasi Dukcapil)  |
| DELETE | `/api/user/v1/member/delete/{id}`      | Hapus member                        |
| GET    | `/api/user/v1/ticket-history`          | Riwayat booking (paginasi)          |
| GET    | `/sys/ping`                            | Health check                        |

> Kenapa auth tanpa versi tapi member pakai `/api/user/v1/...`? Auth = fondasi
> stabil & universal (seperti OAuth `/token`), sengaja dibuat titik tetap tak
> berversi. Detail: `docs/API-CONTRACT.md`.

### Contoh

```bash
# register
curl -X POST localhost:8086/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"budi@mail.com","password":"rahasia123","name":"Budi"}'

# login → simpan authToken
curl -X POST localhost:8086/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"budi@mail.com","password":"rahasia123"}'

# me
curl localhost:8086/auth/me -H "Authorization: Bearer <authToken>"

# tambah member (NIK dicocokkan ke Dukcapil)
curl -X POST localhost:8086/api/user/v1/member/create -H "Authorization: Bearer <authToken>" \
  -H 'Content-Type: application/json' \
  -d '{"nik":"3273000000000001","nama":"Budi Santoso","tglLahir":"1990-01-01","namaIbu":"Siti Aminah"}'
```

Response memakai envelope `{status, message, data}` (`status:"00000"` = sukses).
`valid:false`/kredensial salah = HTTP yang tepat (401/400), bukan 500.

## Konfigurasi (environment variable)

| Env                     | Default                     | Keterangan                         |
|-------------------------|-----------------------------|------------------------------------|
| `APP_PORT`              | `8086`                      | Port HTTP                          |
| `APP_ENV`               | `local`                     | `local` / `preprod` / `production` |
| `LOG_LEVEL` / `LOG_FORMAT` | `info` / `json`          | Logging slog                       |
| `DB_HOST` ... `DB_NAME` | `localhost:5432` `rusuh_booking` | PostgreSQL (bersama)          |
| `REDIS_ADDR`            | `localhost:6379`            | Redis (denylist refresh token)     |
| `USER_JWT_SECRET`       | `dev-user-jwt-...`          | Secret authToken login. **Beda** dari `ACCESS_TOKEN_SECRET` antrian |
| `REFRESH_TOKEN_SECRET`  | `dev-refresh-...`           | Secret refreshToken (harus beda dari `USER_JWT_SECRET`) |
| `USER_JWT_TTL_SECONDS`  | `3600`                      | Umur authToken (detik)             |
| `REFRESH_TOKEN_TTL_SECONDS` | `604800`                | Umur refreshToken (detik)          |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | (kosong)           | Akun admin awal, dibuat saat startup bila belum ada |
| `DUKCAPIL_BASE_URL`     | `http://localhost:8085`     | Alamat dukcapil-mock               |

Service **menolak start** bila: `APP_ENV=production` masih memakai secret default,
`USER_JWT_SECRET` sama dengan `REFRESH_TOKEN_SECRET`, atau hanya salah satu dari
`ADMIN_EMAIL`/`ADMIN_PASSWORD` yang diisi.

## Token & role

- Admin dan customer memakai **authToken yang sama** dari `POST /auth/login`; yang
  membedakan hanya claim `role` (`customer` | `admin`). Role tidak bisa diubah klien
  karena token ditandatangani (HS256).
- authToken ditandatangani `USER_JWT_SECRET`, **terpisah** dari Access Token antrian
  (`ACCESS_TOKEN_SECRET`, milik queue-service). Akibatnya token antrian tidak bisa dipakai
  sebagai token login, dan queue-service tidak memegang secret yang bisa membuat token admin.
  Service lain yang memverifikasi token login (event/booking/payment) harus memakai
  `USER_JWT_SECRET` yang sama.

### Akun admin

Register selalu membuat `customer`, dan **tidak ada endpoint untuk menjadikan admin**.
Akun admin dibuat saat startup dari env:

```bash
ADMIN_EMAIL=admin@rusuh.local ADMIN_PASSWORD='minimal-12-karakter' make run
```

- Email belum terdaftar → akun `admin` dibuat (password di-hash bcrypt).
- Sudah ada sebagai admin → dibiarkan (password **tidak** ditimpa; aman tiap restart & multi-instance).
- Sudah ada sebagai customer → **tidak** dinaikkan otomatis, hanya muncul peringatan di log.
- Credential tidak pernah disimpan di repo/migration; di production isi dari secret manager.

## Perintah Makefile

| Target                | Keterangan                        |
|-----------------------|-----------------------------------|
| `make run`            | Jalankan via `go run ./cmd`       |
| `make build`          | Compile binary ke `bin/api`       |
| `make test`           | Jalankan test                     |
| `make unit-test-coverage` | Test + coverage               |
| `make golint`         | Lint                              |
| `make docker-build`   | Build image `user-service:latest` |

## Struktur

```
cmd/main.go                            # entry point: startup + graceful shutdown
internal/
  config/                              # env → Config (App/Log/DB/JWT/Redis/Dukcapil)
  delivery/server.go                   # composition root (wiring + route + middleware)
  user/
    domain/entity + domain/dto         # data murni & request/response
    port/inbound + port/outbound       # kontrak usecase & repo/client
    application/usecase/               # logika (user, member, history)
    infrastructure/
      db/                              # adapter PostgreSQL (pgx)
      cache/                           # adapter Redis (denylist token)
      gateway/                         # client HTTP ke dukcapil-mock
      http/                            # handler
pkg/
  apinizer/ apperror/                  # envelope response + error domain
  db/ redis/ token/ middleware/        # pool, redis client, JWT, middleware auth
migrations/                            # app_user + member_nik
```
