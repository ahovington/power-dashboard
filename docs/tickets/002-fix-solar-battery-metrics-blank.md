# 002 — Fix solar and battery hero metrics showing zero

## Status
Complete

## Problem

Both the **SOLAR** MetricCard and the **BATTERY** MetricCard persistently show `0` in the dashboard. Investigation against the running stack reveals two independent root causes, described below.

---

## Root cause detail

### Issue A — Fake generator uses UTC time for solar curve (solar = 0 during Sydney daytime)

`SolarWatts` (and `ConsumptionWatts`, `BatteryState`, `BatteryStep`) compute hour-of-day via `timeOfDayHours(t)`, which calls `t.Hour()`. In a Docker container the system clock is UTC, and `time.Now().UTC()` / the seeded `ts` values are all UTC.

Sydney is **UTC+11** in March (AEDT). At UTC 05:36 (the time of the investigation) the local AEDT time is **16:36** — peak solar afternoon. But the sine-curve model sees hour=5.56, which is before civil sunrise, and returns **0 W**.

**Evidence from DB:**
```
-- All live readings (every 5 min) for the past hour — all zero:
2026-03-17 05:36:43 UTC  power_produced=0
2026-03-17 05:31:43 UTC  power_produced=0
...

-- Last non-zero reading (from seed data generated UTC-noon-ish):
2026-03-16 10:38:54 UTC  power_produced=5878
```

UTC 10:38 maps to AEDT 21:38 — evening, after sunset. So the seed data shows solar during UTC "daytime" which is actually Australian nighttime. The solar curve is shifted by +11 hours relative to physical reality.

The same UTC-offset problem affects `ConsumptionWatts` (morning/evening spikes appear at wrong AU hours) and `BatteryState`/`BatteryStep` (integrates from wrong midnight).

### Issue B — `battery_status` table is empty (seed not re-run)

`seedBattery()` was added in ticket 001 but the seed CLI has not been re-run against the live database since then.

```
SELECT COUNT(*) FROM battery_status;  →  0
```

`fetchBatteryStatus` receives `{"status": "no data"}` → returns `null` → `battery?.charge_percentage ?? 0` → MetricCard shows 0.

### Issue C — `.env` FAKE_PROVIDER mismatch (documentation gap)

`.env` contains `FAKE_PROVIDER=false` but the running container has `FAKE_PROVIDER=true`. If someone rebuilds without fixing the `.env`, the fake provider will not start and no new readings will be ingested (Enphase tokens in `.env` are placeholder values). This is not a bug in the code but a documentation/default gap.

---

## Implementation plan

### Step 1 — Add `TimeZone` field to `FakeConfig`

**File:** `backend/pkg/fake/config.go`

```go
type FakeConfig struct {
    Seed         int64
    PeakWatts    int
    LatitudeDeg  float64
    BatteryCapWh int64
    TimeZone     string // IANA name, e.g. "Australia/Sydney"
}

func (c FakeConfig) WithDefaults() FakeConfig {
    if c.PeakWatts == 0  { c.PeakWatts = 6000 }
    if c.LatitudeDeg == 0 { c.LatitudeDeg = -33.87 }
    if c.BatteryCapWh == 0 { c.BatteryCapWh = 13500 }
    if c.TimeZone == "" { c.TimeZone = "Australia/Sydney" }
    return c
}
```

---

### Step 2 — Apply timezone in `generator.go`

**File:** `backend/pkg/fake/generator.go`

Add a `localTime` helper:

```go
// localTime converts t to the timezone configured in cfg.
// Falls back to t unchanged if the timezone cannot be loaded.
func localTime(cfg FakeConfig, t time.Time) time.Time {
    loc, err := time.LoadLocation(cfg.TimeZone)
    if err != nil {
        return t
    }
    return t.In(loc)
}
```

Apply it in every function that extracts hour-of-day:

```go
// SolarWatts — was: hour := timeOfDayHours(t)
func SolarWatts(cfg FakeConfig, t time.Time) int {
    lt := localTime(cfg, t)
    sunrise, sunset := sunriseSunset(cfg.LatitudeDeg, lt)
    hour := timeOfDayHours(lt)
    ...
}

// ConsumptionWatts
func ConsumptionWatts(cfg FakeConfig, t time.Time) int {
    lt := localTime(cfg, t)
    hour := timeOfDayHours(lt)
    ...
}

// BatteryState — midnight must also use local timezone
func BatteryState(cfg FakeConfig, t time.Time) (chargePercent float64, direction string) {
    lt := localTime(cfg, t)
    midnight := time.Date(lt.Year(), lt.Month(), lt.Day(), 0, 0, 0, 0, lt.Location())
    ...
}
```

`BatteryStep` does not compute hour-of-day internally so no change needed there.

---

### Step 3 — Embed IANA timezone database

Alpine Docker images do not include `/usr/share/zoneinfo`, so `time.LoadLocation("Australia/Sydney")` returns an error without an embedded database.

**File:** `backend/cmd/server/main.go`

```go
import _ "time/tzdata"  // embed IANA timezone database for alpine containers
```

**File:** `backend/cmd/seed/main.go`

```go
import _ "time/tzdata"
```

This adds ~450 KB to each binary — acceptable for a server binary.

---

### Step 4 — Thread `FakeTimezone` through config

**File:** `backend/internal/config/config.go`

```go
type Config struct {
    ...
    FakeProvider  bool
    FakeSeed      int64
    FakeTimezone  string  // new
}
```

In `Load()`:
```go
FakeTimezone: getEnv("FAKE_TIMEZONE", "Australia/Sydney"),
```

**File:** `backend/internal/service/provider_factory.go`

```go
Adapter: fake.NewAdapter(fake.FakeConfig{
    Seed:     cfg.FakeSeed,
    TimeZone: cfg.FakeTimezone,
}),
```

---

### Step 5 — Update `.env` and `.env.example`

**File:** `.env`

```
FAKE_PROVIDER=true        # was false — Enphase tokens are placeholders
FAKE_TIMEZONE=Australia/Sydney
```

**File:** `.env.example`

```
FAKE_TIMEZONE=Australia/Sydney   # IANA timezone for fake data generation
```

---

### Step 6 — Re-seed data

After rebuilding the backend, run:

```
just seed
```

This populates both `power_readings` (with timezone-correct solar data) and `battery_status` (which has been empty since ticket 001 added the table).

---

## Files changed

| File | Change |
|---|---|
| `backend/pkg/fake/config.go` | Add `TimeZone string` field; default `"Australia/Sydney"` |
| `backend/pkg/fake/generator.go` | Add `localTime` helper; apply to `SolarWatts`, `ConsumptionWatts`, `BatteryState` |
| `backend/cmd/server/main.go` | `import _ "time/tzdata"` |
| `backend/cmd/seed/main.go` | `import _ "time/tzdata"` |
| `backend/internal/config/config.go` | Add `FakeTimezone string`, load from `FAKE_TIMEZONE` env |
| `backend/internal/service/provider_factory.go` | Pass `FakeTimezone` into `FakeConfig` |
| `.env` | `FAKE_PROVIDER=true`; add `FAKE_TIMEZONE=Australia/Sydney` |
| `.env.example` | Add `FAKE_TIMEZONE` |

---

## Test coverage required

- `fake` package: `TestSolarWatts_UTCMorningIsSydneyAfternoon` — at UTC 05:00, `SolarWatts` with `TimeZone="Australia/Sydney"` returns >0 (afternoon sun); with no timezone / UTC returns 0 (before sunrise)
- `fake` package: `TestSolarWatts_TimezoneRespected` — solar peak occurs near 12:00 local time, not 12:00 UTC
- `fake` package: `TestBatteryState_MidnightReset` — midnight reset uses local timezone, not UTC midnight

---

## Acceptance criteria

- [ ] SOLAR MetricCard shows >0 W during Sydney business hours when fake provider is running
- [ ] BATTERY MetricCard shows charge % (non-zero) after seed is re-run
- [ ] History chart shows solar production aligned with Sydney daytime hours
- [ ] `just seed` populates both `power_readings` and `battery_status` with timezone-correct data
- [ ] `time.LoadLocation("Australia/Sydney")` succeeds inside the Docker backend container
- [ ] All existing tests pass (including `-race`)

---

## NOT in scope

| Item | Rationale |
|---|---|
| Configurable timezone per household | Single-household app; hardcoded default is sufficient |
| DST-aware Enphase data | Enphase returns UTC timestamps; only affects fake provider |
| Frontend timezone display | Out of scope — display formatting is a separate concern |
