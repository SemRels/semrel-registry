package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultTimeout = 10 * time.Second

type Database struct {
	pool *pgxpool.Pool
	dsn  string
}

func Connect(dsn string) (*Database, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("database dsn is required")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	cfg.MinConns = 1
	cfg.MaxConns = 10
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second
	cfg.MaxConnLifetime = 30 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	db := &Database{pool: pool, dsn: dsn}
	if err := db.Health(); err != nil {
		pool.Close()
		return nil, err
	}

	return db, nil
}

func (d *Database) Pool() *pgxpool.Pool {
	if d == nil {
		return nil
	}

	return d.pool
}

func (d *Database) DSN() string {
	if d == nil {
		return ""
	}

	return d.dsn
}

func (d *Database) Close() error {
	if d == nil || d.pool == nil {
		return nil
	}

	d.pool.Close()
	d.pool = nil
	return nil
}

func (d *Database) Health() error {
	if d == nil || d.pool == nil {
		return fmt.Errorf("database is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	if err := d.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	return nil
}

func (d *Database) BeginTx() (pgx.Tx, error) {
	if d == nil || d.pool == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	tx, err := d.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}

	return tx, nil
}
