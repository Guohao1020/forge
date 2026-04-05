package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	config.MaxConns = 20
	config.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	slog.Info("database connected", "host", config.ConnConfig.Host)
	return pool, nil
}

// PoolStats returns connection pool statistics.
type PoolStats struct {
	TotalConns     int32 `json:"totalConns"`
	AcquiredConns  int32 `json:"acquiredConns"`
	IdleConns      int32 `json:"idleConns"`
	MaxConns       int32 `json:"maxConns"`
	AcquireCount   int64 `json:"acquireCount"`
	EmptyAcquire   int64 `json:"emptyAcquireCount"`
}

func GetPoolStats(pool *pgxpool.Pool) PoolStats {
	s := pool.Stat()
	return PoolStats{
		TotalConns:    s.TotalConns(),
		AcquiredConns: s.AcquiredConns(),
		IdleConns:     s.IdleConns(),
		MaxConns:      s.MaxConns(),
		AcquireCount:  s.AcquireCount(),
		EmptyAcquire:  s.EmptyAcquireCount(),
	}
}
