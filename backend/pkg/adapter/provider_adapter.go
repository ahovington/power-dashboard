package adapter

import (
	"context"
	"errors"
	"time"
)

//go:generate mockgen -source=provider_adapter.go -destination=mock_provider_adapter.go -package=adapter

// Sentinel errors returned by all adapter implementations.
// Callers use errors.Is() to distinguish failure modes.
var (
	ErrRateLimited         = errors.New("provider: rate limit exceeded")
	ErrAuthExpired         = errors.New("provider: authentication expired")
	ErrProviderUnavailable = errors.New("provider: service unavailable")
)

type SystemStatus struct {
	ID            string
	Name          string
	Status        string
	PowerProduced int
	PowerConsumed int
}

type PowerMetrics struct {
	Timestamp     time.Time
	PowerProduced int
	PowerConsumed int
	Frequency     float64
	VoltagePhaseA float64
	VoltagePhaseB float64
	VoltagePhaseC float64
}

type DeviceInfo struct {
	ProviderID   string
	DeviceType   string
	Name         string
	SerialNumber string
}

type BatteryStatus struct {
	ChargePercentage float64
	StateOfHealth    int
	PowerFlowing     int
	PowerDirection   string
	CapacityWh       int64
	Temperature      float64
}

type PowerQualityMetrics struct {
	PowerFactorAverage float64
	CurrentPhaseA      float64
	CurrentPhaseB      float64
	CurrentPhaseC      float64
}

// ProviderAdapter is the contract all energy API providers must satisfy.
// Authentication is handled in each adapter's constructor — not this interface.
// No interface{} parameters — all configs are typed.
type ProviderAdapter interface {
	GetSystemStatus(ctx context.Context) (*SystemStatus, error)
	GetPowerMetrics(ctx context.Context, duration time.Duration) ([]PowerMetrics, error)
	GetDeviceList(ctx context.Context) ([]DeviceInfo, error)
	GetBatteryStatus(ctx context.Context) (*BatteryStatus, error)
	GetPowerQuality(ctx context.Context) (*PowerQualityMetrics, error)
}
