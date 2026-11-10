package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb, mr
}

// ----------------------------------------------------------------- TokenStore

func TestTokenStore_ShouldReportRevoked_AfterRevoke(t *testing.T) {
	rdb, mr := newRedis(t)
	s := NewTokenStore(rdb)
	ctx := context.Background()

	before, err := s.IsRevoked(ctx, "tok-1")
	require.NoError(t, err)
	require.NoError(t, s.Revoke(ctx, "tok-1", time.Hour))
	after, err := s.IsRevoked(ctx, "tok-1")
	require.NoError(t, err)

	assert.False(t, before)
	assert.True(t, after)
	assert.True(t, mr.Exists("revoked:refresh:tok-1"))
}

func TestTokenStore_ShouldExpireKey_WhenTTLElapses(t *testing.T) {
	rdb, mr := newRedis(t)
	s := NewTokenStore(rdb)
	ctx := context.Background()
	require.NoError(t, s.Revoke(ctx, "tok-1", 10*time.Second))

	mr.FastForward(11 * time.Second)

	revoked, err := s.IsRevoked(ctx, "tok-1")
	require.NoError(t, err)
	assert.False(t, revoked, "denylist harus hilang sendiri setelah token kedaluwarsa")
}

func TestTokenStore_ShouldUseMinimumTTL_WhenTTLNotPositive(t *testing.T) {
	rdb, mr := newRedis(t)
	s := NewTokenStore(rdb)
	require.NoError(t, s.Revoke(context.Background(), "tok-1", -time.Second))

	assert.Equal(t, time.Minute, mr.TTL("revoked:refresh:tok-1"))
}

func TestTokenStore_ShouldReturnError_WhenRedisDown(t *testing.T) {
	rdb, mr := newRedis(t)
	s := NewTokenStore(rdb)
	mr.Close()

	assert.Error(t, s.Revoke(context.Background(), "t", time.Hour))
	_, err := s.IsRevoked(context.Background(), "t")
	assert.Error(t, err)
}

// ---------------------------------------------------------------- RateLimiter

func TestRateLimiter_ShouldAllowUpToLimit_ThenBlock(t *testing.T) {
	rdb, _ := newRedis(t)
	l := NewRateLimiter(rdb)
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		ok, err := l.Allow(ctx, "k", 3, time.Minute)
		require.NoError(t, err)
		assert.True(t, ok, "panggilan ke-%d masih boleh", i)
	}
	ok, err := l.Allow(ctx, "k", 3, time.Minute)

	require.NoError(t, err)
	assert.False(t, ok, "panggilan ke-4 harus ditolak")
}

func TestRateLimiter_ShouldCountPerKey(t *testing.T) {
	rdb, _ := newRedis(t)
	l := NewRateLimiter(rdb)
	ctx := context.Background()
	_, _ = l.Allow(ctx, "a", 1, time.Minute)

	blocked, _ := l.Allow(ctx, "a", 1, time.Minute)
	other, _ := l.Allow(ctx, "b", 1, time.Minute)

	assert.False(t, blocked)
	assert.True(t, other, "key lain tidak terpengaruh")
}

func TestRateLimiter_ShouldSetExpiryOnWindowKey(t *testing.T) {
	rdb, mr := newRedis(t)
	l := NewRateLimiter(rdb)
	_, err := l.Allow(context.Background(), "k", 5, time.Minute)
	require.NoError(t, err)

	keys := mr.Keys()

	require.Len(t, keys, 1)
	assert.Greater(t, mr.TTL(keys[0]), time.Duration(0), "key jendela harus punya TTL agar tidak menumpuk")
}

func TestRateLimiter_ShouldReturnError_WhenRedisDown(t *testing.T) {
	rdb, mr := newRedis(t)
	l := NewRateLimiter(rdb)
	mr.Close()

	_, err := l.Allow(context.Background(), "k", 1, time.Minute)

	assert.Error(t, err)
}
