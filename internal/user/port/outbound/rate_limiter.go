package outbound

import (
	"context"
	"time"
)

// IRateLimiter membatasi jumlah aksi per key dalam satu jendela waktu.
// Implementasi harus berlaku lintas instance (Cloud Run bisa >1 instance).
type IRateLimiter interface {
	// Allow mencatat satu aksi untuk key dan mengembalikan false bila sudah melewati limit.
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}
