// Package cache (infrastructure) mengimplementasikan outbound.ITokenStore
// memakai Redis. Denylist refresh token disimpan sebagai key dengan TTL.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenStore menyimpan denylist refresh token di Redis.
type TokenStore struct {
	rdb *redis.Client
}

// NewTokenStore menyuntik client Redis (dibuat di delivery/main).
func NewTokenStore(rdb *redis.Client) *TokenStore {
	return &TokenStore{rdb: rdb}
}

// key menyusun nama key Redis untuk sebuah token yang dicabut.
func (s *TokenStore) key(tokenID string) string {
	return fmt.Sprintf("revoked:refresh:%s", tokenID)
}

// Revoke menyimpan key denylist dengan TTL = sisa umur token.
// Value "1" cuma penanda; yang penting keberadaan key + expiry-nya.
func (s *TokenStore) Revoke(ctx context.Context, tokenID string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = time.Minute // token (hampir) kedaluwarsa — simpan sebentar saja
	}
	if err := s.rdb.Set(ctx, s.key(tokenID), "1", ttl).Err(); err != nil {
		return fmt.Errorf("redis revoke: %w", err)
	}
	return nil
}

// IsRevoked true bila key denylist ada.
func (s *TokenStore) IsRevoked(ctx context.Context, tokenID string) (bool, error) {
	n, err := s.rdb.Exists(ctx, s.key(tokenID)).Result()
	if err != nil {
		return false, fmt.Errorf("redis isrevoked: %w", err)
	}
	return n > 0, nil
}
