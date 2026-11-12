// Package db menyediakan koneksi PostgreSQL berupa connection pool (pgxpool).
// Pola sama dengan service lain agar konsisten lintas tim.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/refreshTechRusuh/user-service/internal/config"
)

// NewPool membuat pool koneksi PostgreSQL dari DBConfig, lalu memastikan
// koneksi hidup lewat Ping. Mengembalikan pool siap pakai.
func NewPool(ctx context.Context, cfg config.DBConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}

	poolCfg.MaxConns = toInt32(cfg.MaxOpenConns)
	poolCfg.MinConns = toInt32(cfg.MaxIdleConns)
	poolCfg.MaxConnLifetime = time.Hour
	poolCfg.MaxConnIdleTime = 30 * time.Minute
	poolCfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db: connect: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	return pool, nil
}
