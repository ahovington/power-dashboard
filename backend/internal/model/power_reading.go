package model

import (
	"time"

	"github.com/google/uuid"
)

// PowerReading is a single time-series sample from a device.
// power_net is NOT stored in the DB — always compute it via PowerNet().
type PowerReading struct {
	ID                  int64
	DeviceID            uuid.UUID
	ReadingTimestamp    time.Time // always UTC
	PowerProduced       int       // watts
	PowerConsumed       int       // watts
	EnergyProducedToday int64     // Wh
	EnergyConsumedToday int64     // Wh
	Frequency           float64
	VoltagePhaseA       float64
	VoltagePhaseB       float64
	VoltagePhaseC       float64
	CreatedAt           time.Time
}

// PowerNet returns computed net power. Never store this — compute on read.
func (r *PowerReading) PowerNet() int {
	return r.PowerProduced - r.PowerConsumed
}

type BatteryStatus struct {
	ID               int64
	DeviceID         uuid.UUID
	ReadingTimestamp time.Time
	ChargePercentage float64
	StateOfHealth    int
	PowerFlowing     int
	PowerDirection   string // "charging" | "discharging"
	CapacityWh       int64
	Temperature      float64
	CreatedAt        time.Time
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
	DeviceID      uuid.UUID `json:"device_id"`
	Timestamp     time.Time `json:"timestamp"`
	PowerProduced int       `json:"power_produced"`
	PowerConsumed int       `json:"power_consumed"`
	PowerNet      int       `json:"power_net"`
	BatteryCharge float64   `json:"battery_charge,omitempty"`
}
