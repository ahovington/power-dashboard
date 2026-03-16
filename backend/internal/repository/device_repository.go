package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/power-dashboard/internal/model"
)

type DeviceRepository struct {
	db *pgxpool.Pool
}

func NewDeviceRepository(db *pgxpool.Pool) *DeviceRepository {
	return &DeviceRepository{db: db}
}

// GetActiveDevices returns all non-deleted, active devices across all households.
// Used at startup to create one IngestionService per provider.
func (r *DeviceRepository) GetActiveDevices(ctx context.Context) ([]*model.Device, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, household_id, provider_id, provider_type, device_type,
		       name, serial_number, location, status, created_at, updated_at
		FROM devices
		WHERE status = 'active' AND deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("repository: get active devices: %w", err)
	}
	defer rows.Close()

	var devices []*model.Device
	for rows.Next() {
		d := &model.Device{}
		if err := rows.Scan(
			&d.ID, &d.HouseholdID, &d.ProviderID, &d.ProviderType, &d.DeviceType,
			&d.Name, &d.SerialNumber, &d.Location, &d.Status, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("repository: scan device: %w", err)
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}
