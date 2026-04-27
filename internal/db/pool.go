// Package db owns the pgx pool. We keep the rest of the application out of
// pgx-specific types where reasonable — repositories accept pgxpool.Pool here
// for simplicity, but exposed seams (interfaces) are added in week 4 when we
// introduce read-replica routing.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/umar1593/pulse/internal/config"
)

func NewPool(ctx context.Context, cfg config.DBConfig) (*pgxpool.Pool, error) {
	pgxCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	pgxCfg.MaxConns = cfg.MaxConns
	pgxCfg.MinConns = cfg.MinConns
	pgxCfg.MaxConnLifetime = cfg.MaxConnLifetime
	pgxCfg.HealthCheckPeriod = cfg.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, pgxCfg)
	if err != nil {
		return nil, fmt.Errorf("connect pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	return pool, nil
}
