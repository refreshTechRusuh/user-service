// Package outbound: kontrak penyimpanan refresh-token (untuk logout).
package outbound

import (
	"context"
	"time"
)

// ITokenStore menyimpan daftar refresh token yang DICABUT (revoked/denylist).
// Logout = masukkan refresh token ke denylist dengan TTL = sisa umur token.
// Setelah itu refresh token tsb dianggap tidak sah walau signature masih valid.
type ITokenStore interface {
	// Revoke menandai sebuah refresh token (by jti/hash) sebagai dicabut,
	// dengan ttl = sisa masa berlaku token (agar auto-hapus saat kedaluwarsa).
	Revoke(ctx context.Context, tokenID string, ttl time.Duration) error

	// IsRevoked mengecek apakah token sudah dicabut.
	IsRevoked(ctx context.Context, tokenID string) (bool, error)
}
