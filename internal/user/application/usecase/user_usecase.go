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
	"github.com/refreshTechRusuh/user-service/pkg/token"
)

// UserUsecase memegang dependency (lewat interface outbound, bukan konkret).
// tokenGen: menerbitkan/verifikasi JWT. tokenStore: denylist (sesi yang di-logout
// dan refresh token yang sudah dipakai).
type UserUsecase struct {
	repo       outbound.IUserRepository
	tokenGen   *token.Generator
	tokenStore outbound.ITokenStore
}

// NewUserUsecase menyuntik repository + generator token + token store saat wiring.
func NewUserUsecase(repo outbound.IUserRepository, tokenGen *token.Generator, tokenStore outbound.ITokenStore) *UserUsecase {
	return &UserUsecase{repo: repo, tokenGen: tokenGen, tokenStore: tokenStore}
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

// Login memverifikasi kredensial lalu menerbitkan authToken + refreshToken (JWT).
func (u *UserUsecase) Login(ctx context.Context, req dto.LoginRequest) (dto.LoginResponse, error) {
	user, found, err := u.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		return dto.LoginResponse{}, apperror.Internal(err)
	}
	// Email tidak ada → pakai InvalidCredentials (JANGAN bocorkan "email tak terdaftar").
	if !found {
		return dto.LoginResponse{}, apperror.InvalidCredentials()
	}
	// Password salah → juga InvalidCredentials (pesan sama dgn email tak ada → anti-enumerasi).
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return dto.LoginResponse{}, apperror.InvalidCredentials()
	}

	// Kredensial valid → sesi baru (satu sid per login) → terbitkan token.
	return u.issueTokens(user, token.NewSessionID())
}

// issueTokens menerbitkan pasangan authToken + refreshToken untuk satu sesi (sid).
// Dipakai Login (sid baru) dan Refresh (sid sama dengan sesi sebelumnya).
func (u *UserUsecase) issueTokens(user entity.User, sessionID string) (dto.LoginResponse, error) {
	authToken, err := u.tokenGen.GenerateAuthToken(user.ID, string(user.Role), sessionID)
	if err != nil {
		return dto.LoginResponse{}, apperror.Internal(err)
	}
	refreshToken, err := u.tokenGen.GenerateRefreshToken(user.ID, sessionID)
	if err != nil {
		return dto.LoginResponse{}, apperror.Internal(err)
	}
	return dto.LoginResponse{
		AuthToken:    authToken,
		RefreshToken: refreshToken,
		Role:         user.Role,
		ExpiresIn:    u.tokenGen.AccessTTLSeconds(),
	}, nil
}

// Me mengembalikan profil user berdasarkan userID (dari authToken, di-set middleware).
func (u *UserUsecase) Me(ctx context.Context, userID string) (dto.MeResponse, error) {
	user, found, err := u.repo.FindByID(ctx, userID)
	if err != nil {
		return dto.MeResponse{}, apperror.Internal(err)
	}
	if !found {
		// Token valid tapi user sudah tak ada (dihapus) → perlakukan sbg unauthorized.
		return dto.MeResponse{}, apperror.Unauthorized("Sesi tidak valid")
	}
	return dto.MeResponse{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
	}, nil
}

// Logout mencabut sesi login: sid masuk denylist Redis selama umur maksimum sesi
// (= TTL refreshToken). Setelah itu authToken DAN refreshToken sesi tersebut ditolak.
// Idempoten. sessionID kosong (token lama tanpa sid) → tidak ada yang dicabut, tetap sukses.
func (u *UserUsecase) Logout(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	if err := u.tokenStore.Revoke(ctx, token.SessionRevokeID(sessionID), u.tokenGen.RefreshTTL()); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// Refresh menukar refreshToken dengan authToken + refreshToken baru.
//
// Urutan: verifikasi token → sesi belum di-logout → refresh token belum pernah
// dipakai → user masih ada (role dibaca ulang dari DB) → tandai refresh token lama
// terpakai → terbitkan pasangan baru dengan sid yang sama.
// Refresh token bersifat sekali pakai; memakainya lagi ditolak.
func (u *UserUsecase) Refresh(ctx context.Context, req dto.RefreshRequest) (dto.LoginResponse, error) {
	claims, err := u.tokenGen.VerifyRefreshToken(req.RefreshToken)
	if err != nil {
		return dto.LoginResponse{}, apperror.Unauthorized(msgRefreshInvalid)
	}

	if claims.SessionID != "" {
		loggedOut, err := u.tokenStore.IsRevoked(ctx, token.SessionRevokeID(claims.SessionID))
		if err != nil {
			return dto.LoginResponse{}, apperror.Internal(err)
		}
		if loggedOut {
			return dto.LoginResponse{}, apperror.Unauthorized(msgRefreshInvalid)
		}
	}

	used, err := u.tokenStore.IsRevoked(ctx, req.RefreshToken)
	if err != nil {
		return dto.LoginResponse{}, apperror.Internal(err)
	}
	if used {
		return dto.LoginResponse{}, apperror.Unauthorized(msgRefreshInvalid)
	}

	user, found, err := u.repo.FindByID(ctx, claims.Subject)
	if err != nil {
		return dto.LoginResponse{}, apperror.Internal(err)
	}
	if !found {
		return dto.LoginResponse{}, apperror.Unauthorized(msgRefreshInvalid)
	}

	// Tandai refresh token lama terpakai (TTL = sisa umurnya) SEBELUM menerbitkan yang baru.
	if err := u.tokenStore.Revoke(ctx, req.RefreshToken, time.Until(claims.ExpiresAt.Time)); err != nil {
		return dto.LoginResponse{}, apperror.Internal(err)
	}

	// Refresh token lama (sebelum ada sid) tidak punya sesi: mulai sesi baru.
	sessionID := claims.SessionID
	if sessionID == "" {
		sessionID = token.NewSessionID()
	}
	return u.issueTokens(user, sessionID)
}

// msgRefreshInvalid sengaja satu pesan untuk semua penyebab (token rusak, sesi
// logout, sudah dipakai, user hilang) agar tidak membocorkan alasan ke penyerang.
const msgRefreshInvalid = "Refresh token tidak valid"
