// Package dto berisi bentuk data request/response user (dengan tag JSON).
// Dipisah dari entity mengikuti pola event-service: domain/entity vs domain/dto.
package dto

import (
	"strings"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/entity"
)

// ---- Register ----

// RegisterRequest = data yang dikirim klien untuk mendaftar akun.
type RegisterRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name"`
}

// RegisterResponse = payload sukses register (isi field "data" pada envelope).
// password TIDAK pernah dikembalikan.
type RegisterResponse struct {
	UserID string      `json:"userId"`
	Email  string      `json:"email"`
	Role   entity.Role `json:"role"`
}

// Validate memeriksa RegisterRequest di boundary (sebelum masuk usecase).
// Mengembalikan daftar pesan kesalahan; kosong = valid.
// Keunikan email dicek di usecase/DB (butuh query), bukan di sini.
func (r RegisterRequest) Validate() []string {
	var errs []string
	email := strings.TrimSpace(r.Email)
	if email == "" {
		errs = append(errs, "email wajib diisi")
	} else if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		errs = append(errs, "format email tidak valid")
	}
	if len(r.Password) < 8 {
		errs = append(errs, "password minimal 8 karakter")
	}
	return errs
}

// ---- Login ----

// LoginRequest = data untuk login.
type LoginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse = payload sukses login (isi field "data").
// authToken = JWT login (JANGAN tertukar dengan Access Token antrian dari Queue).
type LoginResponse struct {
	AuthToken    string      `json:"authToken"`
	RefreshToken string      `json:"refreshToken"`
	Role         entity.Role `json:"role"`
	ExpiresIn    int         `json:"expiresIn"` // detik
}

// Validate memeriksa LoginRequest di boundary.
func (r LoginRequest) Validate() []string {
	var errs []string
	if strings.TrimSpace(r.Email) == "" {
		errs = append(errs, "email wajib diisi")
	}
	if r.Password == "" {
		errs = append(errs, "password wajib diisi")
	}
	return errs
}
