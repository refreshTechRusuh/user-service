// Package inbound berisi driving port: kontrak yang DITAWARKAN domain ke
// pemanggil (HTTP handler). Handler bergantung ke interface ini, bukan usecase
// konkret (pola event-service: IEventUsecase).
package inbound

import (
	"context"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/dto"
)

// IUserUsecase adalah kontrak yang dipanggil HTTP handler.
//
// CATATAN: interface Go mewajibkan SEMUA method di sini diimplementasikan usecase
// agar usecase dianggap memenuhi IUserUsecase. Jadi tambahkan method (Login, Me,
// Logout, dst) ke sini HANYA setelah usecase-nya kamu implementasikan — supaya
// tidak bikin gagal compile. Untuk sekarang baru Register.
type IUserUsecase interface {
	// Register mendaftarkan akun baru. Email duplikat -> apperror.EmailTaken().
	Register(ctx context.Context, req dto.RegisterRequest) (dto.RegisterResponse, error)
}
