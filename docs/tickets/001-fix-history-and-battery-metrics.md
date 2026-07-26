# 001 — Fix history chart and implement battery metrics

## Status
Complete

## Problem

Four things appear broken or missing in the dashboard UI. Investigation revealed two root causes:

1. **History chart is blank** — the backend serialises `[]*model.PowerReading` directly to JSON without struct tags, producing PascalCase keys (`ReadingTimestamp`, `PowerProduced`, …). The frontend expects snake_case (`reading_timestamp`, `power_produced`, …). Recharts finds no matching `dataKey` and renders an empty chart.

2. **Battery node shows 0.0 kW forever** — battery data is unimplemented end-to-end. The `EnergyFlowDiagram` and `MetricCard` slots exist in the frontend but are hardcoded to zero. There is no API endpoint, no repository query, no ingestion of battery data, and no real-time SSE payload.

Solar and Grid **do work** — `GetCurrentStatus` hand-maps fields into a `map[string]interface{}` with snake_case keys, so those fields deserialise correctly.

---

## Root cause detail

### Issue A — History JSON serialisation (`handler.go:92`)

```go
// handler.go — GetHistory
writeJSON(w, http.StatusOK, readings)   // readings is []*model.PowerReading
```

`model.PowerReading` has no JSON struct tags:

```go
// model/power_reading.go — current (broken)
type PowerReading struct {
    ReadingTimestamp time.Time  // → JSON: "ReadingTimestamp" ✗
    PowerProduced    int        // → JSON: "PowerProduced"    ✗
    PowerConsumed    int        // → JSON: "PowerConsumed"    ✗
    ...
}
```

Frontend (`HistoryChart.tsx:62`) expects:
```tsx
dataKey="reading_timestamp"   // never found → empty X axis
dataKey="power_produced"      // never found → empty area
dataKey="power_consumed"      // never found → empty area
```

### Issue B — Battery not implemented

**Backend gaps:**

| Layer | Missing |
|---|---|
| `model.BatteryStatus` | No JSON struct tags |
| `ReadingRepository` | No `SaveBatteryStatus` or `GetLatestBatteryStatus` |
| `IngestionService.pollOnce()` | Never calls `adapter.GetBatteryStatus()` |
| `model.PowerEvent` | No `battery_w` or `battery_direction` fields |
| `handler.go` | No battery endpoint |
| `routes.go` | No battery route |

**Frontend gaps:**

| Layer | Missing |
|---|---|
| `types/power.ts` | No `BatteryStatus` interface; `PowerEvent` lacks `battery_w`/`battery_direction` |
| `api.ts` | No `fetchBatteryStatus()` |
| `hooks/` | No `useBatteryStatus` hook |
| `Dashboard.tsx:57-58` | `batteryW={0}` and `batteryDirection="charging"` hardcoded |
| `Dashboard.tsx` | No battery `MetricCard` (charge %) |

---

## Implementation plan

### Step 1 — Add JSON tags to `model.PowerReading` and `model.BatteryStatus`

**File:** `backend/internal/model/power_reading.go`

Add `json` struct tags to all exported fields on both structs. Use snake_case to match the existing API convention established by `GetCurrentStatus`.

```go
type PowerReading struct {
    ID                  int64     `json:"id"`
    DeviceID            uuid.UUID `json:"device_id"`
    ReadingTimestamp    time.Time `json:"reading_timestamp"`
    PowerProduced       int       `json:"power_produced"`
    PowerConsumed       int       `json:"power_consumed"`
    EnergyProducedToday int64     `json:"energy_produced_today"`
    EnergyConsumedToday int64     `json:"energy_consumed_today"`
    Frequency           float64   `json:"frequency"`
    VoltagePhaseA       float64   `json:"voltage_phase_a"`
    VoltagePhaseB       float64   `json:"voltage_phase_b"`
    VoltagePhaseC       float64   `json:"voltage_phase_c"`
    CreatedAt           time.Time `json:"created_at"`
}

type BatteryStatus struct {
    ID               int64     `json:"id"`
    DeviceID         uuid.UUID `json:"device_id"`
    ReadingTimestamp time.Time `json:"reading_timestamp"`
    ChargePercentage float64   `json:"charge_percentage"`
    StateOfHealth    int       `json:"state_of_health"`
    PowerFlowing     int       `json:"power_flowing"`
    PowerDirection   string    `json:"power_direction"`
    CapacityWh       int64     `json:"capacity_wh"`
    Temperature      float64   `json:"temperature"`
    CreatedAt        time.Time `json:"created_at"`
}
```

**Why not a DTO like `GetCurrentStatus` does?** The current status endpoint only returns 5 fields so a hand-written map is acceptable. History returns full structs and would require a separate mapping loop. JSON tags are the idiomatic Go solution and add no runtime overhead.

---

### Step 2 — Add battery fields to `model.PowerEvent`

**File:** `backend/internal/model/power_reading.go`

Add `BatteryW` and `BatteryDirection` to the event. Use pointers so the SSE payload omits them entirely for devices with no battery (rather than sending `0` / `""`):

```go
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
```

---

### Step 3 — Add battery repository methods

**File:** `backend/internal/repository/reading_repository.go`

Add two methods to `ReadingRepository`:

```go
func (r *ReadingRepository) SaveBatteryStatus(ctx context.Context, b *model.BatteryStatus) error
```
Inserts a `battery_status` row with `ON CONFLICT (device_id, reading_timestamp) DO NOTHING`.

```go
func (r *ReadingRepository) GetLatestBatteryStatus(ctx context.Context, deviceID uuid.UUID) (*model.BatteryStatus, error)
```
Returns the single most-recent row for the device, or `nil` if none exists.

Also add `GetLatestBatteryStatus` to the `ReadingQuerier` interface in `power_service.go` so `PowerService` can expose it.

---

### Step 4 — Update `IngestionService` to ingest battery data

**File:** `backend/internal/service/ingestion_service.go`

In `pollOnce()`, after saving the power reading, call `adapter.GetBatteryStatus()`. If it returns a non-nil result, save it and include `BatteryW` and `BatteryDirection` in the `PowerEvent`:

```go
battery, err := s.adapter.GetBatteryStatus(ctx)
if err != nil {
    slog.Warn("ingestion: battery status unavailable", "device_id", s.deviceID, "error", err)
    // non-fatal: continue without battery data
}
if battery != nil {
    if err := s.repo.SaveBatteryStatus(ctx, &model.BatteryStatus{
        DeviceID:         s.deviceID,
        ReadingTimestamp: now,
        ChargePercentage: battery.ChargePercentage,
        StateOfHealth:    battery.StateOfHealth,
        PowerFlowing:     battery.PowerFlowing,
        PowerDirection:   battery.PowerDirection,
        CapacityWh:       battery.CapacityWh,
        Temperature:      battery.Temperature,
    }); err != nil {
        slog.Warn("ingestion: battery save failed", "device_id", s.deviceID, "error", err)
        // non-fatal: event still published without battery fields
    } else {
        event.BatteryW = &battery.PowerFlowing
        event.BatteryDirection = battery.PowerDirection
        charge := battery.ChargePercentage
        event.BatteryCharge = &charge
    }
}
```

Battery errors are non-fatal — log a warning and continue. Devices without batteries (Enphase without Ensemble) return `nil` from `GetBatteryStatus()` already.

---

### Step 5 — Add `GET /api/v1/power/battery` endpoint

**File:** `backend/internal/api/handler.go`

New handler method:

```go
func (h *Handler) GetBatteryStatus(w http.ResponseWriter, r *http.Request) {
    deviceID, err := uuid.Parse(r.URL.Query().Get("device_id"))
    if err != nil {
        writeError(w, http.StatusBadRequest, "invalid device_id")
        return
    }
    b, err := h.power.GetLatestBatteryStatus(r.Context(), deviceID)
    if err != nil {
        slog.Error("handler: get battery status", "error", err)
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }
    if b == nil {
        writeJSON(w, http.StatusOK, map[string]string{"status": "no data"})
        return
    }
    writeJSON(w, http.StatusOK, b)
}
```

**File:** `backend/internal/api/routes.go`

Register the route:

```go
r.Get("/api/v1/power/battery", handler.GetBatteryStatus)
```

`PowerServicer` interface needs the new method added:

```go
GetLatestBatteryStatus(ctx context.Context, deviceID uuid.UUID) (*model.BatteryStatus, error)
```

---

### Step 6 — Update frontend types

**File:** `frontend/src/types/power.ts`

```typescript
export interface BatteryStatus {
  device_id: string;
  reading_timestamp: string;
  charge_percentage: number;   // 0–100
  state_of_health: number;
  power_flowing: number;       // watts
  power_direction: 'charging' | 'discharging';
  capacity_wh: number;
}

// Update PowerEvent:
export interface PowerEvent {
  device_id: string;
  timestamp: string;
  power_produced: number;
  power_consumed: number;
  power_net: number;
  battery_charge?: number;
  battery_w?: number;
  battery_direction?: 'charging' | 'discharging';
}
```

---

### Step 7 — Add `fetchBatteryStatus` to `api.ts`

**File:** `frontend/src/services/api.ts`

```typescript
export async function fetchBatteryStatus(deviceId: string): Promise<BatteryStatus | null> {
  const res = await fetch(`${BASE_URL}/api/v1/power/battery?device_id=${deviceId}`);
  if (!res.ok) throw new Error(`fetchBatteryStatus: HTTP ${res.status}`);
  const data = await res.json();
  if (data.status === 'no data') return null;
  return data;
}
```

---

### Step 8 — Add `useBatteryStatus` hook

**File:** `frontend/src/hooks/useBatteryStatus.ts` (new file)

Follows the same pattern as `usePowerStatus`. Polls once on mount and updates on SSE events that contain `battery_w`:

```typescript
export function useBatteryStatus(deviceId: string, latestEvent?: PowerEvent | null) {
  const [battery, setBattery] = useState<BatteryStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    fetchBatteryStatus(deviceId)
      .then(b => { setBattery(b); setLoading(false); setError(null); })
      .catch(err => { setLoading(false); setError(String(err)); });
  }, [deviceId]);

  useEffect(() => {
    // Fix: was `!latestEvent?.battery_w == null` which never fires (operator precedence bug)
    if (latestEvent?.battery_w == null) return;
    // Update power_flowing and direction from SSE without a full re-fetch
    setBattery(prev => prev ? {
      ...prev,
      power_flowing: latestEvent.battery_w!,
      power_direction: latestEvent.battery_direction ?? prev.power_direction,
    } : prev);
  }, [latestEvent]);

  return { battery, loading, error };
}
```

---

### Step 9 — Wire battery into `Dashboard.tsx`

**File:** `frontend/src/components/Dashboard.tsx`

1. Call `useBatteryStatus(deviceId, latestEvent)`.
2. Replace hardcoded values passed to `EnergyFlowDiagram`:

```tsx
// Before:
batteryW={0}
batteryDirection="charging"

// After:
batteryW={battery?.power_flowing ?? 0}
batteryDirection={battery?.power_direction ?? 'discharging'}
```

3. Add a battery `MetricCard` to the metric strip:

```tsx
<MetricCard
  label="BATTERY"
  value={battery ? Math.round(battery.charge_percentage) : 0}
  unit="%"
  accent="green"
/>
```

---

### Step 10 — Update seed CLI

**File:** `backend/cmd/seed/main.go`

The seed CLI currently only inserts `power_readings`. Add a second `CopyFrom` call to bulk-insert `battery_status` rows using `fake.BatteryState()` for every reading timestamp. This ensures the history and battery endpoints both return data in the demo environment.

**Implementation note — avoid O(n²) battery state:** `fake.BatteryState(t)` is defined as an integrator that simulates charge from midnight in 5-minute steps. Calling it naively for each row (e.g. iterating from midnight on every call) makes the seed O(n²) for large day counts. Instead, thread an explicit `chargePercent float64` accumulator through the battery rows loop and step it forward using `fake.BatteryStep(prev, solar, consumption)` at each interval. This keeps the seed O(n).

---

## Files changed

| File | Change |
|---|---|
| `backend/internal/model/power_reading.go` | Add JSON tags to `PowerReading`, `BatteryStatus`; add battery fields to `PowerEvent` |
| `backend/internal/repository/reading_repository.go` | Add `SaveBatteryStatus`, `GetLatestBatteryStatus` |
| `backend/internal/service/power_service.go` | Add `GetLatestBatteryStatus` to interface and service |
| `backend/internal/service/ingestion_service.go` | Call `GetBatteryStatus`, save + publish in event |
| `backend/internal/api/handler.go` | Add `GetBatteryStatus` handler; add to `PowerServicer` interface |
| `backend/internal/api/routes.go` | Register `GET /api/v1/power/battery` |
| `backend/cmd/seed/main.go` | Seed `battery_status` rows |
| `frontend/src/types/power.ts` | Add `BatteryStatus`; extend `PowerEvent` |
| `frontend/src/services/api.ts` | Add `fetchBatteryStatus` |
| `frontend/src/hooks/useBatteryStatus.ts` | New hook |
| `frontend/src/components/Dashboard.tsx` | Wire battery hook; fix hardcoded values; add MetricCard |

## Test coverage required

- `ReadingRepository`: unit tests for `SaveBatteryStatus` and `GetLatestBatteryStatus`
- `IngestionService`: test that `pollOnce` saves battery status when adapter returns non-nil, and skips silently when nil
- `Handler`: test `GetBatteryStatus` returns 200 with data, 200 with `{status: "no data"}` when none
- **`Handler` (JSON serialization):** test that `GET /api/v1/power/history` returns JSON with `reading_timestamp` (snake_case), not `ReadingTimestamp` (PascalCase). This is the regression test for the JSON struct tag fix — without it, removing tags would silently break the chart with no compile-time error.
- `useBatteryStatus`: test initial fetch and SSE-driven update
- `Dashboard`: test battery MetricCard renders with charge_percentage

## NOT in scope

| Item | Rationale |
|---|---|
| Enphase battery API wiring (`GetBatteryStatus` in enphase adapter) | Enphase Ensemble endpoint requires separate research; fake provider is sufficient for this ticket |
| Historical battery chart (battery % over time as area chart) | New chart component; not needed to meet the acceptance criteria |
| Battery capacity / state-of-health MetricCard | One MetricCard (charge %) is sufficient; capacity is a setup-time value not a live metric |
| `TIMESTAMPTZ` schema migration | Correctness fix but a separate one-way door; tracked in TODOS.md |
| Multi-device battery aggregation | Single-device only; multi-device is a separate architectural concern |

---

## Acceptance criteria

- [ ] History chart renders solar and consumption area lines for all interval buttons (hour / day / week / month)
- [ ] Battery node in `EnergyFlowDiagram` shows real wattage and animates in the correct direction
- [ ] Battery `MetricCard` shows charge percentage
- [ ] Battery data updates in real-time via SSE when the fake provider is running
- [ ] All existing tests continue to pass
- [ ] Seed CLI populates both `power_readings` and `battery_status` tables

---

## Plan review summary (eng review — SMALL CHANGE mode)

- **Step 0 (Scope Challenge):** Chose SMALL CHANGE — scope is appropriate, 11 files touched is within bounds
- **Architecture:** 1 issue — battery save error handling. **Resolution:** log `slog.Warn` on error, continue without battery fields (non-fatal). Applied to Step 4.
- **Code Quality:** 1 issue — SSE guard operator precedence bug in `useBatteryStatus`. **Resolution:** `if (latestEvent?.battery_w == null) return;` with `loading`/`error` state. Applied to Step 8.
- **Tests:** 1 issue — no regression test for JSON struct tag fix. **Resolution:** added `Handler (JSON serialization)` test requirement to Test coverage section.
- **Performance:** 1 issue — O(n²) battery state in seed CLI. **Resolution:** thread iterative `chargePercent` accumulator through loop. Applied to Step 10.
- **NOT in scope:** 5 items explicitly deferred (see above)
- **Failure modes:** 0 critical gaps — all new error paths are either tested, logged, or return structured errors to the caller
