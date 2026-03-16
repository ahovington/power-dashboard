package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/power-dashboard/internal/model"
	"github.com/yourusername/power-dashboard/internal/repository"
)

func setupDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping integration test")
	}

	// Run migrations up; tear them down after the test.
	m, err := migrate.New("file://../../migrations", url)
	require.NoError(t, err)
	require.NoError(t, m.Up())
	t.Cleanup(func() {
		m.Down() //nolint: errcheck — cleanup only
	})

	db, err := pgxpool.New(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(db.Close)
	return db
}

func TestSaveReading_Persists(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewReadingRepository(db)

	deviceID := uuid.New()
	ts := time.Now().UTC().Truncate(time.Second)

	err := repo.SaveReading(context.Background(), &model.PowerReading{
		DeviceID: deviceID, ReadingTimestamp: ts,
		PowerProduced: 5000, PowerConsumed: 3000,
	})
	require.NoError(t, err)

	readings, err := repo.GetLatestReadings(context.Background(), deviceID, 1)
	require.NoError(t, err)
	require.Len(t, readings, 1)
	assert.Equal(t, 5000, readings[0].PowerProduced)
	assert.Equal(t, ts, readings[0].ReadingTimestamp.UTC())
}

func TestSaveReading_DuplicateIsIgnored(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewReadingRepository(db)

	deviceID := uuid.New()
	ts := time.Now().UTC()
	reading := &model.PowerReading{DeviceID: deviceID, ReadingTimestamp: ts, PowerProduced: 1000}

	require.NoError(t, repo.SaveReading(context.Background(), reading))
	require.NoError(t, repo.SaveReading(context.Background(), reading),
		"ON CONFLICT DO NOTHING should silently skip duplicate")

	readings, _ := repo.GetLatestReadings(context.Background(), deviceID, 10)
	assert.Len(t, readings, 1, "exactly one row should exist")
}

func TestGetLatestReadings_EmptyReturnsNilNotError(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewReadingRepository(db)

	readings, err := repo.GetLatestReadings(context.Background(), uuid.New(), 10)
	assert.NoError(t, err)
	assert.Empty(t, readings)
}

func TestGetAggregatedReadings_HourlyBuckets(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewReadingRepository(db)

	deviceID := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour)

	for i := 0; i < 3; i++ {
		_ = repo.SaveReading(context.Background(), &model.PowerReading{
			DeviceID:         deviceID,
			ReadingTimestamp: base.Add(time.Duration(i*5) * time.Minute),
			PowerProduced:    1000, PowerConsumed: 500,
		})
	}

	buckets, err := repo.GetAggregatedReadings(
		context.Background(), deviceID, "hour",
		base.Add(-time.Minute), base.Add(time.Hour),
	)
	require.NoError(t, err)
	assert.Len(t, buckets, 1, "all readings fall in one hour bucket")
	assert.Equal(t, 1000, buckets[0].PowerProduced)
}
