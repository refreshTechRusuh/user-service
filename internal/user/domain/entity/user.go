// Package entity berisi entity inti domain user (data murni, tanpa tag JSON).
// Mengikuti pola event-service: internal/{domain}/domain/entity/.
package entity

import "time"

// Role = peran akun. customer = pembeli biasa, admin = pengelola event.
type Role string

const (
	RoleCustomer Role = "customer"
	RoleAdmin    Role = "admin"
)

// User = satu akun, seperti disimpan di tabel app_user.
// PasswordHash = hasil bcrypt (JANGAN simpan password plain, JANGAN kirim balik ke klien).
type User struct {
	ID           string
	Email        string
	PasswordHash string
	Role         Role
	CreatedAt    time.Time
}
