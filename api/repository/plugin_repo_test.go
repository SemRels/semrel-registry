package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPostgresRepositoryWithoutDatabaseFailsFast(t *testing.T) {
	repo := NewPluginRepository(nil)
	_, err := repo.GetAll(context.Background(), 10, 0)
	require.ErrorContains(t, err, "database is not initialized")
}
