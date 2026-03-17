package fake

import (
	"math"
	"math/rand"
	"time"
)

const degreesToRad = math.Pi / 180

// SolarWatts returns instantaneous solar production in watts at time t.
// Zero outside civil sunrise/sunset; sine-curve peak at solar noon.
func SolarWatts(cfg FakeConfig, t time.Time) int {
	sunrise, sunset := sunriseSunset(cfg.LatitudeDeg, t)
	hour := timeOfDayHours(t)
	if hour <= sunrise || hour >= sunset {
		return 0
	}
	ratio := (hour - sunrise) / (sunset - sunrise)
	base := float64(cfg.PeakWatts) * math.Sin(math.Pi*ratio)
	return int(math.Max(0, base+Jitter(cfg.Seed, t, base*0.05)))
}

// ConsumptionWatts returns household power consumption in watts at time t.
// Models overnight base load, morning and evening spikes.
func ConsumptionWatts(cfg FakeConfig, t time.Time) int {
	hour := timeOfDayHours(t)
	base := piecewiseConsumption(hour)
	return int(math.Max(100, base+Jitter(cfg.Seed+1, t, base*0.05)))
}

// piecewiseConsumption returns watts for fractional hour-of-day (0–24).
func piecewiseConsumption(hour float64) float64 {
	switch {
	case hour < 6:
		return 800
	case hour < 9:
		return lerp(hour, 6, 9, 800, 2000) // morning ramp
	case hour < 11:
		return lerp(hour, 9, 11, 2000, 1000) // settle after breakfast
	case hour < 17:
		return 1000 // midday plateau
	case hour < 20:
		return lerp(hour, 17, 20, 1000, 3000) // evening ramp (cooking, EV)
	case hour < 22:
		return lerp(hour, 20, 22, 3000, 1500) // post-dinner wind-down
	default:
		return lerp(hour, 22, 24, 1500, 800) // toward overnight base
	}
}

// BatteryState returns (chargePercent, direction) at time t.
// Integrates net power in 5-minute steps from midnight to approximate state of charge.
func BatteryState(cfg FakeConfig, t time.Time) (chargePercent float64, direction string) {
	midnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	const step = 5 * time.Minute
	const stepHours = 5.0 / 60.0

	charge := 50.0 // start each day at 50%
	cap := float64(cfg.BatteryCapWh)

	for ts := midnight; ts.Before(t); ts = ts.Add(step) {
		produced := float64(SolarWatts(cfg, ts))
		consumed := float64(ConsumptionWatts(cfg, ts))
		deltaPercent := ((produced - consumed) * stepHours / cap) * 100
		charge = math.Max(10, math.Min(95, charge+deltaPercent))
	}

	produced := SolarWatts(cfg, t)
	consumed := ConsumptionWatts(cfg, t)
	if produced >= consumed {
		direction = "charging"
	} else {
		direction = "discharging"
	}
	return charge, direction
}

// BatteryStep advances the battery state by one interval step.
// Use this in loops instead of BatteryState to avoid O(n²) re-integration from midnight.
// prev is the charge percentage from the previous step (start with 50.0 for midnight).
// intervalHours is the step size in fractional hours (e.g. 5.0/60.0 for 5-minute steps).
func BatteryStep(cfg FakeConfig, prev float64, produced, consumed int, intervalHours float64) (chargePercent float64, direction string) {
	cap := float64(cfg.BatteryCapWh)
	deltaPercent := (float64(produced-consumed) * intervalHours / cap) * 100
	chargePercent = math.Max(10, math.Min(95, prev+deltaPercent))
	if produced >= consumed {
		direction = "charging"
	} else {
		direction = "discharging"
	}
	return
}

// Frequency returns grid frequency (60 Hz) with small deterministic jitter.
func Frequency(cfg FakeConfig, t time.Time) float64 {
	return 60.0 + Jitter(cfg.Seed+5, t, 0.05)
}

// VoltagePhase returns nominal 240 V with small deterministic jitter.
// phase is 0, 1, or 2 for phases A, B, C.
func VoltagePhase(seed int64, t time.Time, phase int) float64 {
	return 240.0 + Jitter(seed+int64(phase)*7, t, 2.0)
}

// PowerFactor returns a stable power factor near 0.97.
func PowerFactor(cfg FakeConfig, t time.Time) float64 {
	return 0.97 + Jitter(cfg.Seed+6, t, 0.01)
}

// sunriseSunset returns approximate civil sunrise and sunset as fractional hours
// (0–24) for the given latitude and the date of t.
// Uses standard solar declination formula; no external dependency.
func sunriseSunset(latDeg float64, t time.Time) (sunrise, sunset float64) {
	dayOfYear := float64(t.YearDay())
	declination := 23.45 * math.Sin(degreesToRad*(360.0/365.0)*(dayOfYear-81))
	lat := latDeg * degreesToRad
	decl := declination * degreesToRad

	cosH := -math.Tan(lat) * math.Tan(decl)
	if cosH < -1 {
		return 0, 24 // polar day
	}
	if cosH > 1 {
		return 12, 12 // polar night
	}
	h := math.Acos(cosH) * (180 / math.Pi) / 15
	return 12 - h, 12 + h
}

// timeOfDayHours returns fractional hour of day (0–24) for t.
func timeOfDayHours(t time.Time) float64 {
	return float64(t.Hour()) + float64(t.Minute())/60.0 + float64(t.Second())/3600.0
}

// Jitter returns a deterministic offset in [-maxAbs, +maxAbs] derived from seed
// and a 5-minute bucket of t — stable within the same bucket, varies across buckets.
func Jitter(seed int64, t time.Time, maxAbs float64) float64 {
	bucket := t.Unix() / 300 // 5-minute stability buckets
	r := rand.New(rand.NewSource(seed + bucket))
	return (r.Float64()*2 - 1) * maxAbs
}

// lerp linearly interpolates v from [x0,x1] to [y0,y1].
func lerp(v, x0, x1, y0, y1 float64) float64 {
	return y0 + (v-x0)/(x1-x0)*(y1-y0)
}
