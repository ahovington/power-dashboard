package fake

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var testCfg = FakeConfig{Seed: 42, PeakWatts: 6000, LatitudeDeg: -33.87, BatteryCapWh: 13500}

func TestSolarWatts_ZeroAtMidnight(t *testing.T) {
	midnight := time.Date(2025, 6, 21, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, 0, SolarWatts(testCfg, midnight))
}

func TestSolarWatts_ZeroAt3AM(t *testing.T) {
	t3am := time.Date(2025, 6, 21, 3, 0, 0, 0, time.UTC)
	assert.Equal(t, 0, SolarWatts(testCfg, t3am))
}

func TestSolarWatts_PeakAtNoon(t *testing.T) {
	noon := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC) // southern hemisphere summer
	w := SolarWatts(testCfg, noon)
	assert.InDelta(t, 6000, w, 600, "expected near-peak production at solar noon")
}

func TestSolarWatts_NonNegative(t *testing.T) {
	for hour := 0; hour < 24; hour++ {
		ts := time.Date(2025, 6, 21, hour, 0, 0, 0, time.UTC)
		assert.GreaterOrEqual(t, SolarWatts(testCfg, ts), 0)
	}
}

func TestConsumptionWatts_NightBaseLoad(t *testing.T) {
	t2am := time.Date(2025, 6, 21, 2, 0, 0, 0, time.UTC)
	w := ConsumptionWatts(testCfg, t2am)
	assert.InDelta(t, 800, w, 100)
}

func TestConsumptionWatts_MorningSpikeHigherThanNight(t *testing.T) {
	night := time.Date(2025, 6, 21, 2, 0, 0, 0, time.UTC)
	morning := time.Date(2025, 6, 21, 7, 30, 0, 0, time.UTC)
	assert.Greater(t, ConsumptionWatts(testCfg, morning), ConsumptionWatts(testCfg, night))
}

func TestConsumptionWatts_EveningSpikeHighest(t *testing.T) {
	midday := time.Date(2025, 6, 21, 12, 0, 0, 0, time.UTC)
	evening := time.Date(2025, 6, 21, 19, 0, 0, 0, time.UTC)
	assert.Greater(t, ConsumptionWatts(testCfg, evening), ConsumptionWatts(testCfg, midday))
}

func TestConsumptionWatts_AlwaysPositive(t *testing.T) {
	for hour := 0; hour < 24; hour++ {
		ts := time.Date(2025, 6, 21, hour, 0, 0, 0, time.UTC)
		assert.Greater(t, ConsumptionWatts(testCfg, ts), 0)
	}
}

func TestBatteryState_ChargeWithinBounds(t *testing.T) {
	for _, hour := range []int{0, 3, 6, 9, 12, 15, 18, 21, 23} {
		ts := time.Date(2025, 6, 21, hour, 0, 0, 0, time.UTC)
		charge, _ := BatteryState(testCfg, ts)
		assert.GreaterOrEqual(t, charge, 10.0, "hour %d: charge below minimum", hour)
		assert.LessOrEqual(t, charge, 95.0, "hour %d: charge above maximum", hour)
	}
}

func TestBatteryState_ChargingAtSolarNoon(t *testing.T) {
	// Southern hemisphere summer noon: peak solar >> consumption
	noon := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	_, dir := BatteryState(testCfg, noon)
	assert.Equal(t, "charging", dir)
}

func TestBatteryState_DischargingAtNight(t *testing.T) {
	night := time.Date(2025, 6, 21, 22, 0, 0, 0, time.UTC)
	_, dir := BatteryState(testCfg, night)
	assert.Equal(t, "discharging", dir)
}

func TestJitter_Deterministic(t *testing.T) {
	ts := time.Date(2025, 6, 21, 12, 0, 0, 0, time.UTC)
	j1 := Jitter(42, ts, 100.0)
	j2 := Jitter(42, ts, 100.0)
	assert.Equal(t, j1, j2, "same seed + timestamp must produce same Jitter")
}

func TestJitter_WithinBounds(t *testing.T) {
	ts := time.Date(2025, 6, 21, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 100; i++ {
		j := Jitter(int64(i), ts, 50.0)
		assert.GreaterOrEqual(t, j, -50.0)
		assert.LessOrEqual(t, j, 50.0)
	}
}

func TestJitter_VariesWithSeed(t *testing.T) {
	ts := time.Date(2025, 6, 21, 12, 0, 0, 0, time.UTC)
	j1 := Jitter(1, ts, 100.0)
	j2 := Jitter(2, ts, 100.0)
	assert.NotEqual(t, j1, j2)
}

func TestSunriseSunset_SummerLongerDay(t *testing.T) {
	summer := time.Date(2025, 12, 21, 0, 0, 0, 0, time.UTC) // southern hemisphere summer
	winter := time.Date(2025, 6, 21, 0, 0, 0, 0, time.UTC)
	rSummer, sSummer := sunriseSunset(-33.87, summer)
	rWinter, sWinter := sunriseSunset(-33.87, winter)
	summerDayLength := sSummer - rSummer
	winterDayLength := sWinter - rWinter
	assert.Greater(t, summerDayLength, winterDayLength, "summer day should be longer than winter")
}
