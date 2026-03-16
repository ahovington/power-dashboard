package fake

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/ahovingtonpower-dashboard/pkg/adapter"
)

// Adapter is a ProviderAdapter that generates deterministic synthetic data.
// It requires no external API credentials and is safe for concurrent use.
type Adapter struct {
	cfg FakeConfig
}

// NewAdapter creates a fake provider adapter.
// If cfg.Seed is 0, a random seed is chosen at construction (non-deterministic across restarts).
func NewAdapter(cfg FakeConfig) *Adapter {
	cfg = cfg.WithDefaults()
	if cfg.Seed == 0 {
		cfg.Seed = rand.New(rand.NewSource(time.Now().UnixNano())).Int63()
	}
	return &Adapter{cfg: cfg}
}

func (a *Adapter) GetSystemStatus(ctx context.Context) (*adapter.SystemStatus, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	now := time.Now()
	return &adapter.SystemStatus{
		ID:            "fake-system-001",
		Name:          "Demo Home",
		Status:        "normal",
		PowerProduced: SolarWatts(a.cfg, now),
		PowerConsumed: ConsumptionWatts(a.cfg, now),
	}, nil
}

func (a *Adapter) GetPowerMetrics(ctx context.Context, duration time.Duration) ([]adapter.PowerMetrics, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	now := time.Now()
	start := now.Add(-duration)
	const step = 5 * time.Minute

	var metrics []adapter.PowerMetrics
	for ts := start; !ts.After(now); ts = ts.Add(step) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		metrics = append(metrics, adapter.PowerMetrics{
			Timestamp:     ts,
			PowerProduced: SolarWatts(a.cfg, ts),
			PowerConsumed: ConsumptionWatts(a.cfg, ts),
			Frequency:     Frequency(a.cfg, ts),
			VoltagePhaseA: VoltagePhase(a.cfg.Seed, ts, 0),
			VoltagePhaseB: VoltagePhase(a.cfg.Seed, ts, 1),
			VoltagePhaseC: VoltagePhase(a.cfg.Seed, ts, 2),
		})
	}
	return metrics, nil
}

func (a *Adapter) GetDeviceList(ctx context.Context) ([]adapter.DeviceInfo, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return []adapter.DeviceInfo{
		{
			ProviderID:   "fake-001",
			DeviceType:   "solar_inverter",
			Name:         "Demo Solar Inverter",
			SerialNumber: "FAKE-SN-001",
		},
	}, nil
}

func (a *Adapter) GetBatteryStatus(ctx context.Context) (*adapter.BatteryStatus, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	now := time.Now()
	charge, dir := BatteryState(a.cfg, now)
	net := SolarWatts(a.cfg, now) - ConsumptionWatts(a.cfg, now)
	powerFlowing := int(math.Abs(float64(net)))
	return &adapter.BatteryStatus{
		ChargePercentage: charge,
		StateOfHealth:    94,
		PowerFlowing:     powerFlowing,
		PowerDirection:   dir,
		CapacityWh:       a.cfg.BatteryCapWh,
		Temperature:      25.0 + Jitter(a.cfg.Seed+10, now, 2.0),
	}, nil
}

func (a *Adapter) GetPowerQuality(ctx context.Context) (*adapter.PowerQualityMetrics, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	now := time.Now()
	pf := PowerFactor(a.cfg, now)
	produced := float64(SolarWatts(a.cfg, now))
	current := produced / (240.0 * pf * 3) // split across 3 phases
	return &adapter.PowerQualityMetrics{
		PowerFactorAverage: pf,
		CurrentPhaseA:      current + Jitter(a.cfg.Seed+11, now, 0.5),
		CurrentPhaseB:      current + Jitter(a.cfg.Seed+12, now, 0.5),
		CurrentPhaseC:      current + Jitter(a.cfg.Seed+13, now, 0.5),
	}, nil
}
