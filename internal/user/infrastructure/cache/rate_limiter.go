package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter = fixed-window counter di Redis (berlaku lintas instance).
type RateLimiter struct {
	rdb *redis.Client
}

func NewRateLimiter(rdb *redis.Client) *RateLimiter {
	return &RateLimiter{rdb: rdb}
}

// Allow menaikkan counter jendela saat ini; false bila melewati limit.
// Error Redis dikembalikan ke pemanggil, yang memutuskan fail-open/closed.
func (l *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	slot := time.Now().Unix() / int64(window.Seconds())
	k := "ratelimit:" + key + ":" + strconv.FormatInt(slot, 10)

	n, err := l.rdb.Incr(ctx, k).Result()
	if err != nil {
		return false, fmt.Errorf("redis ratelimit incr: %w", err)
	}
	if n == 1 {
		// Sisa umur sedikit lebih panjang dari jendela agar counter tak hilang di tengah jendela.
		if err := l.rdb.Expire(ctx, k, window+10*time.Second).Err(); err != nil {
			return false, fmt.Errorf("redis ratelimit expire: %w", err)
		}
	}
	return n <= int64(limit), nil
}
