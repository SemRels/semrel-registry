package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const PluginWriteAdvisoryLockKey int64 = 0x53454d52454c

func LockPluginWrites(ctx context.Context, tx pgx.Tx) error {
	if tx == nil {
		return fmt.Errorf("plugin write transaction is required")
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`,
		PluginWriteAdvisoryLockKey); err != nil {
		return fmt.Errorf("lock plugin writes: %w", err)
	}
	return nil
}
