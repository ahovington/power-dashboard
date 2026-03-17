package fake

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// FakeDeviceID is the fixed UUID used by the fake provider for both the live
// adapter and the seed CLI. All synthetic data rows use this device ID.
var FakeDeviceID = uuid.MustParse("fa4eda7a-0000-4000-a000-000000000001")

// FakeUserID and FakeHouseholdID are the fixed UUIDs for seed CLI fixtures.
var (
	FakeUserID      = uuid.MustParse("fa4e0001-0000-4000-a000-000000000001")
	FakeHouseholdID = uuid.MustParse("fa4e0002-0000-4000-a000-000000000001")
)

// FakeConfig controls the synthetic data generator.
type FakeConfig struct {
	// Seed for deterministic jitter. 0 = use time.Now().UnixNano() at construction.
	Seed int64
	// PeakWatts is the solar system peak output (default 6000 W).
	PeakWatts int
	// LatitudeDeg is used for sunrise/sunset calculation (default -33.87, Sydney).
	LatitudeDeg float64
	// BatteryCapWh is the battery capacity in watt-hours (default 13500 Wh).
	BatteryCapWh int64
	// TimeZone is the IANA timezone name used to interpret hour-of-day for solar
	// and consumption curves (default "Australia/Sydney"). An invalid name panics
	// in WithDefaults — like uuid.MustParse, fail fast at startup.
	TimeZone string
}

// WithDefaults returns a copy of c with zero values replaced by sensible defaults.
// Panics if TimeZone is set to an unrecognised IANA name — this is intentional:
// a bad timezone produces silently wrong solar curves, so fail fast at startup.
func (c FakeConfig) WithDefaults() FakeConfig {
	if c.PeakWatts == 0 {
		c.PeakWatts = 6000
	}
	if c.LatitudeDeg == 0 {
		c.LatitudeDeg = -33.87 // Sydney
	}
	if c.BatteryCapWh == 0 {
		c.BatteryCapWh = 13500
	}
	if c.TimeZone == "" {
		c.TimeZone = "Australia/Sydney"
	}
	if _, err := time.LoadLocation(c.TimeZone); err != nil {
		panic(fmt.Sprintf("fake: invalid TimeZone %q: %v", c.TimeZone, err))
	}
	return c
}
