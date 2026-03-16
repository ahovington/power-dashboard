package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/power-dashboard/internal/model"
)

type ReadingRepository struct {
	db *pgxpool.Pool
}

func NewReadingRepository(db *pgxpool.Pool) *ReadingRepository {
	return &ReadingRepository{db: db}
}

// SaveReading persists one reading. ON CONFLICT DO NOTHING prevents duplicates
// when the ingestion goroutine re-polls an overlapping time window after restart.
func (r *ReadingRepository) SaveReading(ctx context.Context, reading *model.PowerReading) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO power_readings (
			device_id, reading_timestamp,
			power_produced, power_consumed,
			energy_produced_today, energy_consumed_today,
			frequency, voltage_phase_a, voltage_phase_b, voltage_phase_c
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (device_id, reading_timestamp) DO NOTHING`,
		reading.DeviceID,
		reading.ReadingTimestamp.UTC(), // always store UTC
		reading.PowerProduced, reading.PowerConsumed,
		reading.EnergyProducedToday, reading.EnergyConsumedToday,
		reading.Frequency,
		reading.VoltagePhaseA, reading.VoltagePhaseB, reading.VoltagePhaseC,
	)
	if err != nil {
		return fmt.Errorf("repository: save reading: %w", err)
	}
	return nil
}

func (r *ReadingRepository) GetLatestReadings(ctx context.Context, deviceID uuid.UUID, limit int) ([]*model.PowerReading, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, device_id, reading_timestamp,
		       power_produced, power_consumed,
		       energy_produced_today, energy_consumed_today,
		       frequency, voltage_phase_a, voltage_phase_b, voltage_phase_c,
		       created_at
		FROM power_readings
		WHERE device_id = $1
		ORDER BY reading_timestamp DESC
		LIMIT $2`,
		deviceID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("repository: get latest readings: %w", err)
	}
	defer rows.Close()

	var readings []*model.PowerReading
	for rows.Next() {
		r2 := &model.PowerReading{}
		if err := rows.Scan(
			&r2.ID, &r2.DeviceID, &r2.ReadingTimestamp,
			&r2.PowerProduced, &r2.PowerConsumed,
			&r2.EnergyProducedToday, &r2.EnergyConsumedToday,
			&r2.Frequency, &r2.VoltagePhaseA, &r2.VoltagePhaseB, &r2.VoltagePhaseC,
			&r2.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("repository: scan reading: %w", err)
		}
		readings = append(readings, r2)
	}
	return readings, rows.Err()
}

// GetAggregatedReadings returns bucketed averages for chart queries.
// interval must be one of: "hour", "day", "week", "month".
// Always use this for time-range chart data — never return raw rows.
func (r *ReadingRepository) GetAggregatedReadings(
	ctx context.Context,
	deviceID uuid.UUID,
	interval string,
	start, end time.Time,
) ([]*model.PowerReading, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			DATE_TRUNC($1, reading_timestamp) AS bucket,
			AVG(power_produced)::INT,
			AVG(power_consumed)::INT,
			SUM(energy_produced_today),
			SUM(energy_consumed_today)
		FROM power_readings
		WHERE device_id = $2
		  AND reading_timestamp BETWEEN $3 AND $4
		GROUP BY bucket
		ORDER BY bucket ASC`,
		interval, deviceID, start.UTC(), end.UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("repository: get aggregated readings: %w", err)
	}
	defer rows.Close()

	var readings []*model.PowerReading
	for rows.Next() {
		r2 := &model.PowerReading{}
		if err := rows.Scan(
			&r2.ReadingTimestamp,
			&r2.PowerProduced, &r2.PowerConsumed,
			&r2.EnergyProducedToday, &r2.EnergyConsumedToday,
		); err != nil {
			return nil, fmt.Errorf("repository: scan aggregated reading: %w", err)
		}
		readings = append(readings, r2)
	}
	return readings, rows.Err()
}
