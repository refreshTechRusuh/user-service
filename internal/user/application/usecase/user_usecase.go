// Package usecase berisi logika bisnis user-service.
// Lapis ini TAHU domain + port, tapi TIDAK tahu HTTP/DB (hexagonal).
package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/dto"
	"github.com/refreshTechRusuh/user-service/internal/user/domain/entity"
	"github.com/refreshTechRusuh/user-service/internal/user/port/outbound"
	"github.com/refreshTechRusuh/user-service/pkg/apperror"
)

// UserUsecase memegang dependency (lewat interface outbound, bukan konkret).
type UserUsecase struct {
	repo outbound.IUserRepository
}

// NewUserUsecase menyuntik repository saat wiring (di delivery).
func NewUserUsecase(repo outbound.IUserRepository) *UserUsecase {
	return &UserUsecase{repo: repo}
}

// Register mendaftarkan akun baru.
// Langkah: cek email -> hash password -> simpan user baru -> balas response.
func (u *UserUsecase) Register(ctx context.Context, req dto.RegisterRequest) (dto.RegisterResponse, error) {
	// 1. Cek email sudah dipakai? (error teknis diteruskan, jangan diabaikan)
	_, found, err := u.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		return dto.RegisterResponse{}, apperror.Internal(err)
	}
	if found {
		return dto.RegisterResponse{}, apperror.EmailTaken()
	}

	// 2. Hash password (bcrypt). GenerateFromPassword return ([]byte, error).
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return dto.RegisterResponse{}, apperror.Internal(err)
	}

	// 3. Bangun entity user baru (id UUID, role default customer, waktu sekarang).
	user := entity.User{
		ID:           uuid.NewString(),
		Email:        req.Email,
		PasswordHash: string(hash),
		Role:         entity.RoleCustomer,
		CreatedAt:    time.Now().UTC(),
	}

	// 4. Simpan. Create mengembalikan apperror.EmailTaken() bila race lolos ke DB.
	if err := u.repo.Create(ctx, user); err != nil {
		return dto.RegisterResponse{}, err
	}

	// 5. Balas response (tanpa password).
	return dto.RegisterResponse{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
	}, nil
}
