// cmd/seed inserts synthetic historical data into the database using the fake
// provider generator. Run this once to populate power_readings so the history
// API endpoint returns meaningful chart data without a real Enphase system.
//
// Usage:
//
//	go run ./cmd/seed [--days=30] [--interval=5m] [--seed=42]
//
// Environment:
//
//	DATABASE_URL  PostgreSQL connection string (required, same as the server)
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/ahovingtonpower-dashboard/pkg/fake"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	days := flag.Int("days", 30, "number of days of history to generate")
	interval := flag.Duration("interval", 5*time.Minute, "reading interval (e.g. 5m, 15m)")
	seed := flag.Int64("seed", 42, "deterministic seed (0 = random)")
	flag.Parse()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("db connect", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("db ping", "error", err)
		os.Exit(1)
	}

	if err := ensureFixtures(ctx, pool); err != nil {
		slog.Error("fixtures", "error", err)
		os.Exit(1)
	}

	cfg := fake.FakeConfig{Seed: *seed}

	end := time.Now().UTC().Truncate(*interval)
	start := end.Add(-time.Duration(*days) * 24 * time.Hour)

	slog.Info("seeding", "start", start.Format(time.RFC3339), "end", end.Format(time.RFC3339), "interval", interval, "seed", *seed)

	inserted, err := seedReadings(ctx, pool, cfg, start, end, *interval)
	if err != nil {
		slog.Error("seed readings", "error", err)
		os.Exit(1)
	}

	slog.Info("done", "rows_inserted", inserted, "device_id", fake.FakeDeviceID)
	fmt.Printf("Seeded %d power readings for device %s\n", inserted, fake.FakeDeviceID)
	fmt.Printf("Use device_id=%s in API requests.\n", fake.FakeDeviceID)
}

// ensureFixtures creates the seed user, household, and device rows if they do
// not already exist. Safe to call multiple times (ON CONFLICT DO NOTHING).
func ensureFixtures(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash)
		VALUES ($1, 'seed@example.com', 'not-a-real-hash')
		ON CONFLICT (id) DO NOTHING`,
		fake.FakeUserID,
	)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO households (id, user_id, name, timezone)
		VALUES ($1, $2, 'Demo Home', 'Australia/Sydney')
		ON CONFLICT (id) DO NOTHING`,
		fake.FakeHouseholdID, fake.FakeUserID,
	)
	if err != nil {
		return fmt.Errorf("insert household: %w", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO devices (id, household_id, provider_id, provider_type, device_type, name, serial_number, status)
		VALUES ($1, $2, 'fake-001', 'fake', 'solar_inverter', 'Demo Solar Inverter', 'FAKE-SN-001', 'active')
		ON CONFLICT (id) DO NOTHING`,
		fake.FakeDeviceID, fake.FakeHouseholdID,
	)
	if err != nil {
		return fmt.Errorf("insert device: %w", err)
	}

	slog.Info("fixtures ready", "device_id", fake.FakeDeviceID)
	return nil
}

// seedReadings bulk-inserts power_readings rows from start to end at the given
// interval. Uses pgx CopyFrom for efficient batch insertion.
// Returns the number of rows actually inserted.
func seedReadings(ctx context.Context, pool *pgxpool.Pool, cfg fake.FakeConfig, start, end time.Time, interval time.Duration) (int64, error) {
	cfg = cfg.WithDefaults()

	columns := []string{
		"device_id", "reading_timestamp",
		"power_produced", "power_consumed",
		"energy_produced_today", "energy_consumed_today",
		"frequency", "voltage_phase_a", "voltage_phase_b", "voltage_phase_c",
	}

	// Build all rows in memory. 30 days * 288 readings/day = 8,640 rows — fine.
	var rows [][]any

	var (
		prevDay        time.Time
		energyProduced float64
		energyConsumed float64
	)

	intervalHours := interval.Hours()

	for ts := start; !ts.After(end); ts = ts.Add(interval) {
		day := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, ts.Location())
		if day != prevDay {
			energyProduced = 0
			energyConsumed = 0
			prevDay = day
		}

		produced := fake.SolarWatts(cfg, ts)
		consumed := fake.ConsumptionWatts(cfg, ts)
		energyProduced += float64(produced) * intervalHours
		energyConsumed += float64(consumed) * intervalHours

		rows = append(rows, []any{
			fake.FakeDeviceID,
			ts.UTC(),
			produced,
			consumed,
			int64(energyProduced),
			int64(energyConsumed),
			fake.Frequency(cfg, ts),
			fake.VoltagePhase(cfg.Seed, ts, 0),
			fake.VoltagePhase(cfg.Seed, ts, 1),
			fake.VoltagePhase(cfg.Seed, ts, 2),
		})
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return 0, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	n, err := conn.Conn().CopyFrom(
		ctx,
		pgx.Identifier{"power_readings"},
		columns,
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return 0, fmt.Errorf("copy from: %w", err)
	}
	return n, nil
}
