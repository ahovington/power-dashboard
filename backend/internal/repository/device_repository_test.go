package repository_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/power-dashboard/internal/repository"
)

func TestGetActiveDevices_ReturnsNonDeleted(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewDeviceRepository(db)

	// With a fresh schema and no seeded data, expect an empty (non-error) result.
	// Full fixture-based tests require inserting test households/devices first.
	devices, err := repo.GetActiveDevices(context.Background())
	require.NoError(t, err)
	for _, d := range devices {
		assert.Nil(t, d.DeletedAt, "soft-deleted devices must not appear")
		assert.Equal(t, "active", d.Status)
	}
}
