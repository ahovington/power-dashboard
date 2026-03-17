package fake

import (
	"testing"
	"time"
	_ "time/tzdata" // embed IANA timezone database so tests pass on alpine/CI

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

// ---- Timezone-aware tests ----
// Sydney is UTC+11 in March (AEDT). These tests confirm that the generator
// uses local time for hour-of-day, not the raw UTC timestamp.

var sydneyCfg = FakeConfig{Seed: 42, PeakWatts: 6000, LatitudeDeg: -33.87, BatteryCapWh: 13500, TimeZone: "Australia/Sydney"}

func TestSolarWatts_UTCMorningIsSydneyAfternoon(t *testing.T) {
	// UTC 05:00 on 17 Mar 2026 = AEDT 16:00 — peak afternoon solar.
	// Without timezone the curve sees hour=5 (before sunrise) → 0.
	// With Sydney timezone the curve sees hour=16 → positive production.
	ts := time.Date(2026, 3, 17, 5, 0, 0, 0, time.UTC)

	assert.Equal(t, 0, SolarWatts(testCfg, ts), "UTC hour 5 without TZ should be before Sydney sunrise")
	assert.Greater(t, SolarWatts(sydneyCfg, ts), 0, "UTC 05:00 should map to Sydney afternoon and produce solar")
}

func TestSolarWatts_TimezoneRespected(t *testing.T) {
	// Solar peak should be near 12:00 local (Sydney), not 12:00 UTC.
	// In March, AEDT = UTC+11: Sydney noon = UTC 01:00; UTC noon = Sydney 23:00.
	utcNoon := time.Date(2026, 3, 17, 12, 0, 0, 0, time.UTC)    // Sydney 23:00 — night
	sydneyNoon := time.Date(2026, 3, 17, 1, 0, 0, 0, time.UTC) // Sydney 12:00 — peak

	assert.Equal(t, 0, SolarWatts(sydneyCfg, utcNoon), "UTC noon is Sydney 23:00 — no solar production expected")
	assert.Greater(t, SolarWatts(sydneyCfg, sydneyNoon), 0, "UTC 01:00 is Sydney noon — solar production expected")
}

func TestBatteryState_MidnightReset(t *testing.T) {
	// UTC 14:00 on 17 Mar = AEDT 01:00 on 18 Mar — one hour past Sydney midnight.
	// With Sydney TZ the integration window is only 1 hour (no solar, ~800 W drain),
	// so charge should remain close to the 50% starting point.
	ts := time.Date(2026, 3, 17, 14, 0, 0, 0, time.UTC)

	chargeSydney, _ := BatteryState(sydneyCfg, ts)
	assert.InDelta(t, 50.0, chargeSydney, 10.0, "charge should be near 50% just after local Sydney midnight")
}

func TestConsumptionWatts_MorningPeakInLocalTime(t *testing.T) {
	// UTC 20:00 = AEDT 07:00 — morning ramp (hour 6–9, 800–2000 W).
	// Without timezone hour=20 is the evening ramp (≥3000 W peak).
	ts := time.Date(2026, 3, 17, 20, 0, 0, 0, time.UTC)

	w := ConsumptionWatts(sydneyCfg, ts)
	assert.GreaterOrEqual(t, w, 800, "UTC 20:00 / Sydney 07:00 should be at or above morning base load")
	assert.Less(t, w, 2200, "UTC 20:00 / Sydney 07:00 should still be in morning ramp, not evening peak")
}

func TestFakeConfig_InvalidTimezonePanics(t *testing.T) {
	assert.Panics(t, func() {
		FakeConfig{Seed: 42, TimeZone: "Not/A/Real/Zone"}.WithDefaults()
	}, "WithDefaults should panic for an unrecognised IANA timezone name")
}
