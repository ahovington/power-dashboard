package model

import (
	"time"

	"github.com/google/uuid"
)

// PowerReading is a single time-series sample from a device.
// power_net is NOT stored in the DB — always compute it via PowerNet().
type PowerReading struct {
	ID                  int64     `json:"id"`
	DeviceID            uuid.UUID `json:"device_id"`
	ReadingTimestamp    time.Time `json:"reading_timestamp"` // always UTC
	PowerProduced       int       `json:"power_produced"`    // watts
	PowerConsumed       int       `json:"power_consumed"`    // watts
	EnergyProducedToday int64     `json:"energy_produced_today"` // Wh
	EnergyConsumedToday int64     `json:"energy_consumed_today"` // Wh
	Frequency           float64   `json:"frequency"`
	VoltagePhaseA       float64   `json:"voltage_phase_a"`
	VoltagePhaseB       float64   `json:"voltage_phase_b"`
	VoltagePhaseC       float64   `json:"voltage_phase_c"`
	CreatedAt           time.Time `json:"created_at"`
}

// PowerNet returns computed net power. Never store this — compute on read.
func (r *PowerReading) PowerNet() int {
	return r.PowerProduced - r.PowerConsumed
}

type BatteryStatus struct {
	ID               int64     `json:"id"`
	DeviceID         uuid.UUID `json:"device_id"`
	ReadingTimestamp time.Time `json:"reading_timestamp"`
	ChargePercentage float64   `json:"charge_percentage"`
	StateOfHealth    int       `json:"state_of_health"`
	PowerFlowing     int       `json:"power_flowing"`
	PowerDirection   string    `json:"power_direction"` // "charging" | "discharging"
	CapacityWh       int64     `json:"capacity_wh"`
	Temperature      float64   `json:"temperature"`
	CreatedAt        time.Time `json:"created_at"`
}

type Device struct {
	ID           uuid.UUID
	HouseholdID  uuid.UUID
	ProviderID   string
	ProviderType string
	DeviceType   string
	Name         string
	SerialNumber string
	Location     string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

// PowerEvent is published on the internal event bus after each successful ingestion cycle.
// The SSE hub fans this out to all connected browser clients.
//
//	IngestionService ──► eventBus chan ──► Hub.Broadcast ──► SSE clients
type PowerEvent struct {
	DeviceID         uuid.UUID `json:"device_id"`
	Timestamp        time.Time `json:"timestamp"`
	PowerProduced    int       `json:"power_produced"`
	PowerConsumed    int       `json:"power_consumed"`
	PowerNet         int       `json:"power_net"`
	BatteryCharge    *float64  `json:"battery_charge,omitempty"`
	BatteryW         *int      `json:"battery_w,omitempty"`
	BatteryDirection string    `json:"battery_direction,omitempty"`
}
