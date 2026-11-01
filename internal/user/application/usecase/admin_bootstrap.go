package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/entity"
	"github.com/refreshTechRusuh/user-service/pkg/apperror"
)

// BootstrapResult = apa yang dilakukan EnsureAdmin (untuk log startup).
type BootstrapResult string

const (
	BootstrapSkipped BootstrapResult = "SKIPPED" // ADMIN_EMAIL kosong
	BootstrapCreated BootstrapResult = "CREATED" // akun admin baru dibuat
	BootstrapExists  BootstrapResult = "EXISTS"  // email sudah ada sebagai admin → tidak diubah
	BootstrapNotAdm  BootstrapResult = "NOT_ADMIN"
)

// EnsureAdmin memastikan akun admin awal ada. Dipanggil SEKALI saat startup
// (bukan endpoint HTTP — sengaja tidak ada API "jadikan admin").
//
// Aturan:
//   - email kosong            → dilewati (admin tidak dikelola lewat env)
//   - email belum terdaftar   → buat akun role=admin (password di-hash bcrypt)
//   - email sudah admin       → tidak diubah (password TIDAK ditimpa)
//   - email sudah customer    → TIDAK dinaikkan otomatis; dilaporkan agar dicek manual
//     (mencegah akun orang lain naik jadi admin hanya karena env salah ketik)
//
// Idempoten: aman dijalankan tiap startup & di banyak instance sekaligus
// (bila dua instance membuat bersamaan, satu kalah di UNIQUE email → dianggap EXISTS).
func (u *UserUsecase) EnsureAdmin(ctx context.Context, email, password string) (BootstrapResult, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return BootstrapSkipped, nil
	}
	if !strings.Contains(email, "@") {
		return "", fmt.Errorf("ADMIN_EMAIL tidak valid")
	}
	if len(password) < 12 {
		return "", fmt.Errorf("ADMIN_PASSWORD minimal 12 karakter")
	}

	existing, found, err := u.repo.FindByEmail(ctx, email)
	if err != nil {
		return "", fmt.Errorf("cek akun admin: %w", err)
	}
	if found {
		if existing.Role == entity.RoleAdmin {
			return BootstrapExists, nil
		}
		return BootstrapNotAdm, nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password admin: %w", err)
	}
	err = u.repo.Create(ctx, entity.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: string(hash),
		Role:         entity.RoleAdmin,
		CreatedAt:    time.Now().UTC(),
	})
	if err != nil {
		var ae *apperror.AppError
		if errors.As(err, &ae) && ae.Code == apperror.ErrEmailTaken {
			return BootstrapExists, nil // instance lain membuat duluan
		}
		return "", fmt.Errorf("buat akun admin: %w", err)
	}
	return BootstrapCreated, nil
}
