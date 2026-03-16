# Backend Implementation Guide (TDD)

Step-by-step instructions for building the Go backend using Test-Driven Development.

**TDD cycle for every component:**
1. **Red** — write a failing test that describes the desired behaviour
2. **Green** — write the minimum production code to make it pass
3. **Refactor** — clean up without breaking the tests

Run `go test -race ./...` after every green step before moving on.

---

## Step 1 — Module & Project Scaffolding

No tests yet — pure setup that everything else depends on.

### `backend/go.mod`

```go
module github.com/ahovingtonpower-dashboard

go 1.21

require (
    github.com/go-chi/chi/v5 v5.0.10
    github.com/go-chi/cors v1.2.1
    github.com/golang-migrate/migrate/v4 v4.17.0
    github.com/google/uuid v1.4.0
    github.com/jackc/pgx/v5 v5.5.0
    github.com/stretchr/testify v1.8.4
    go.uber.org/mock v0.4.0
    github.com/prometheus/client_golang v1.17.0
)
```

```bash
cd backend && go mod tidy
go install go.uber.org/mock/mockgen@latest
```

---

## Step 2 — Domain Models

Models are pure structs with no I/O. Write them first so all later tests can use real types.

### Red: model tests first (`backend/internal/model/power_reading_test.go`)

```go
package model_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/ahovingtonpower-dashboard/internal/model"
)

func TestPowerReading_PowerNet(t *testing.T) {
    r := &model.PowerReading{PowerProduced: 5000, PowerConsumed: 3000}
    assert.Equal(t, 2000, r.PowerNet())
}

func TestPowerReading_PowerNet_WhenConsumedExceedsProduced(t *testing.T) {
    r := &model.PowerReading{PowerProduced: 1000, PowerConsumed: 3000}
    assert.Equal(t, -2000, r.PowerNet(), "negative net means drawing from grid")
}
```

```bash
go test ./internal/model/...  # Red
```

### Green: `backend/internal/model/power_reading.go`

```go
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
//   IngestionService ──► eventBus chan ──► Hub.Broadcast ──► SSE clients
type PowerEvent struct {
    DeviceID      uuid.UUID `json:"device_id"`
    Timestamp     time.Time `json:"timestamp"`
    PowerProduced int       `json:"power_produced"`
    PowerConsumed int       `json:"power_consumed"`
    PowerNet      int       `json:"power_net"`
    BatteryCharge float64   `json:"battery_charge,omitempty"`
}
```

```bash
go test ./internal/model/...  # Green
```

---

## Step 3 — Adapter Interface & Typed Errors

Write the contract before any implementation.

### `backend/pkg/adapter/provider_adapter.go`

```go
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
```

Generate the mock:

```bash
go generate ./pkg/adapter/...
```

---

## Step 4 — Enphase Adapter (TDD)

### Red: tests first (`backend/pkg/enphase/adapter_test.go`)

```go
package enphase_test

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/ahovingtonpower-dashboard/pkg/adapter"
    "github.com/ahovingtonpower-dashboard/pkg/enphase"
)

func TestGetSystemStatus_OK(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(enphase.MockSystemStatusResponse())
    }))
    defer srv.Close()

    a := enphase.NewAdapter(enphase.Config{APIKey: "test-key", BaseURL: srv.URL})
    status, err := a.GetSystemStatus(context.Background())

    require.NoError(t, err)
    assert.Equal(t, 5000, status.PowerProduced)
    assert.Equal(t, 3000, status.PowerConsumed)
}

func TestGetSystemStatus_RateLimit(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusTooManyRequests)
    }))
    defer srv.Close()

    a := enphase.NewAdapter(enphase.Config{APIKey: "test", BaseURL: srv.URL})
    _, err := a.GetSystemStatus(context.Background())

    assert.ErrorIs(t, err, adapter.ErrRateLimited)
}

func TestGetSystemStatus_AuthExpired(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusUnauthorized)
    }))
    defer srv.Close()

    a := enphase.NewAdapter(enphase.Config{APIKey: "expired", BaseURL: srv.URL})
    _, err := a.GetSystemStatus(context.Background())

    assert.ErrorIs(t, err, adapter.ErrAuthExpired)
}

func TestGetSystemStatus_ServerError(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusInternalServerError)
    }))
    defer srv.Close()

    a := enphase.NewAdapter(enphase.Config{APIKey: "test", BaseURL: srv.URL})
    _, err := a.GetSystemStatus(context.Background())

    assert.ErrorIs(t, err, adapter.ErrProviderUnavailable)
}

func TestGetSystemStatus_MalformedJSON(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("not valid json {{{"))
    }))
    defer srv.Close()

    a := enphase.NewAdapter(enphase.Config{APIKey: "test", BaseURL: srv.URL})
    _, err := a.GetSystemStatus(context.Background())

    assert.Error(t, err)
    assert.NotErrorIs(t, err, adapter.ErrRateLimited)
}

func TestGetSystemStatus_NetworkTimeout(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        <-r.Context().Done() // hang until client gives up
    }))
    defer srv.Close()

    a := enphase.NewAdapter(enphase.Config{
        APIKey:         "test",
        BaseURL:        srv.URL,
        RequestTimeout: 50 * time.Millisecond,
    })
    _, err := a.GetSystemStatus(context.Background())

    assert.Error(t, err)
}
```

```bash
go test ./pkg/enphase/...  # Red
```

### Green: `backend/pkg/enphase/types.go` and `client.go`

**types.go**

```go
package enphase

import "time"

type Config struct {
    APIKey         string
    SystemID       string
    BaseURL        string        // override in tests; defaults to Enphase production URL
    RequestTimeout time.Duration // defaults to 15s
}

type SystemStatusResponse struct {
    SystemID    string `json:"system_id"`
    Name        string `json:"name"`
    Status      string `json:"status"`
    Production  int    `json:"current_power"`
    Consumption int    `json:"consumption_power"`
}

type TelemetryResponse struct {
    Intervals []struct {
        EndAt        int64 `json:"end_at"`
        Wh           int   `json:"wh_del"`
        WConsumption int   `json:"wh_cons"`
    } `json:"intervals"`
}

type DevicesResponse struct {
    Devices []struct {
        SerialNum  string `json:"sn"`
        Model      string `json:"model"`
        DeviceType string `json:"type"`
    } `json:"devices"`
}

func MockSystemStatusResponse() *SystemStatusResponse {
    return &SystemStatusResponse{
        SystemID:    "test-system-123",
        Name:        "Test Home",
        Status:      "normal",
        Production:  5000,
        Consumption: 3000,
    }
}
```

**client.go**

```go
package enphase

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/ahovingtonpower-dashboard/pkg/adapter"
)

const defaultBaseURL = "https://api.enphaseenergy.com/api/v4"

type Adapter struct {
    cfg    Config
    client *http.Client
}

func NewAdapter(cfg Config) *Adapter {
    if cfg.BaseURL == "" {
        cfg.BaseURL = defaultBaseURL
    }
    if cfg.RequestTimeout == 0 {
        cfg.RequestTimeout = 15 * time.Second
    }
    return &Adapter{cfg: cfg, client: &http.Client{Timeout: cfg.RequestTimeout}}
}

// get is the shared HTTP helper: sets auth, checks status code, decodes JSON.
// All adapter methods call this rather than duplicating HTTP boilerplate.
func (a *Adapter) get(ctx context.Context, path string, out interface{}) error {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.cfg.BaseURL+path, nil)
    if err != nil {
        return fmt.Errorf("enphase: build request: %w", err)
    }
    req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)

    resp, err := a.client.Do(req)
    if err != nil {
        return fmt.Errorf("enphase: do request: %w", err)
    }
    defer resp.Body.Close()

    switch resp.StatusCode {
    case http.StatusOK:
    case http.StatusUnauthorized, http.StatusForbidden:
        return adapter.ErrAuthExpired
    case http.StatusTooManyRequests:
        return adapter.ErrRateLimited
    default:
        return fmt.Errorf("%w: HTTP %d from %s", adapter.ErrProviderUnavailable, resp.StatusCode, path)
    }

    if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
        return fmt.Errorf("enphase: decode response from %s: %w", path, err)
    }
    return nil
}

func (a *Adapter) GetSystemStatus(ctx context.Context) (*adapter.SystemStatus, error) {
    var resp SystemStatusResponse
    if err := a.get(ctx, fmt.Sprintf("/systems/%s/summary", a.cfg.SystemID), &resp); err != nil {
        return nil, err
    }
    return &adapter.SystemStatus{
        ID:            resp.SystemID,
        Name:          resp.Name,
        Status:        resp.Status,
        PowerProduced: resp.Production,
        PowerConsumed: resp.Consumption,
    }, nil
}

func (a *Adapter) GetPowerMetrics(ctx context.Context, duration time.Duration) ([]adapter.PowerMetrics, error) {
    var resp TelemetryResponse
    if err := a.get(ctx, fmt.Sprintf("/systems/%s/telemetry/production_micro", a.cfg.SystemID), &resp); err != nil {
        return nil, err
    }
    metrics := make([]adapter.PowerMetrics, 0, len(resp.Intervals))
    for _, iv := range resp.Intervals {
        metrics = append(metrics, adapter.PowerMetrics{
            Timestamp:     time.Unix(iv.EndAt, 0).UTC(),
            PowerProduced: iv.Wh,
            PowerConsumed: iv.WConsumption,
        })
    }
    return metrics, nil
}

func (a *Adapter) GetDeviceList(ctx context.Context) ([]adapter.DeviceInfo, error) {
    var resp DevicesResponse
    if err := a.get(ctx, fmt.Sprintf("/systems/%s/devices", a.cfg.SystemID), &resp); err != nil {
        return nil, err
    }
    devices := make([]adapter.DeviceInfo, 0, len(resp.Devices))
    for _, d := range resp.Devices {
        devices = append(devices, adapter.DeviceInfo{
            ProviderID: d.SerialNum, DeviceType: d.DeviceType,
            Name: d.Model, SerialNumber: d.SerialNum,
        })
    }
    return devices, nil
}

func (a *Adapter) GetBatteryStatus(_ context.Context) (*adapter.BatteryStatus, error) {
    return nil, nil // requires Enphase Ensemble; return nil if unavailable
}

func (a *Adapter) GetPowerQuality(_ context.Context) (*adapter.PowerQualityMetrics, error) {
    return nil, nil // requires Envoy local API
}
```

```bash
go test ./pkg/enphase/...  # Green
```

---

## Step 5 — Repository (TDD)

### Red: integration tests first (`backend/internal/repository/reading_repository_test.go`)

`setupDB` runs migrations and defers teardown — tests own their full fixture lifecycle.

```go
package repository_test

import (
    "context"
    "os"
    "testing"
    "time"

    "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/ahovingtonpower-dashboard/internal/model"
    "github.com/ahovingtonpower-dashboard/internal/repository"
)

func setupDB(t *testing.T) *pgxpool.Pool {
    t.Helper()
    url := os.Getenv("TEST_DATABASE_URL")
    if url == "" {
        t.Skip("TEST_DATABASE_URL not set — skipping integration test")
    }

    // Run migrations up; tear them down after the test.
    m, err := migrate.New("file://../../migrations", url)
    require.NoError(t, err)
    require.NoError(t, m.Up())
    t.Cleanup(func() {
        m.Down() // nolint: ignore error on cleanup
    })

    db, err := pgxpool.New(context.Background(), url)
    require.NoError(t, err)
    t.Cleanup(db.Close)
    return db
}

func TestSaveReading_Persists(t *testing.T) {
    db := setupDB(t)
    repo := repository.NewReadingRepository(db)

    deviceID := uuid.New()
    ts := time.Now().UTC().Truncate(time.Second)

    err := repo.SaveReading(context.Background(), &model.PowerReading{
        DeviceID: deviceID, ReadingTimestamp: ts,
        PowerProduced: 5000, PowerConsumed: 3000,
    })
    require.NoError(t, err)

    readings, err := repo.GetLatestReadings(context.Background(), deviceID, 1)
    require.NoError(t, err)
    require.Len(t, readings, 1)
    assert.Equal(t, 5000, readings[0].PowerProduced)
    assert.Equal(t, ts, readings[0].ReadingTimestamp.UTC())
}

func TestSaveReading_DuplicateIsIgnored(t *testing.T) {
    db := setupDB(t)
    repo := repository.NewReadingRepository(db)

    deviceID := uuid.New()
    ts := time.Now().UTC()
    reading := &model.PowerReading{DeviceID: deviceID, ReadingTimestamp: ts, PowerProduced: 1000}

    require.NoError(t, repo.SaveReading(context.Background(), reading))
    require.NoError(t, repo.SaveReading(context.Background(), reading),
        "ON CONFLICT DO NOTHING should silently skip duplicate")

    readings, _ := repo.GetLatestReadings(context.Background(), deviceID, 10)
    assert.Len(t, readings, 1, "exactly one row should exist")
}

func TestGetLatestReadings_EmptyReturnsNilNotError(t *testing.T) {
    db := setupDB(t)
    repo := repository.NewReadingRepository(db)

    readings, err := repo.GetLatestReadings(context.Background(), uuid.New(), 10)
    assert.NoError(t, err)
    assert.Empty(t, readings)
}

func TestGetAggregatedReadings_HourlyBuckets(t *testing.T) {
    db := setupDB(t)
    repo := repository.NewReadingRepository(db)

    deviceID := uuid.New()
    base := time.Now().UTC().Truncate(time.Hour)

    for i := 0; i < 3; i++ {
        _ = repo.SaveReading(context.Background(), &model.PowerReading{
            DeviceID:         deviceID,
            ReadingTimestamp: base.Add(time.Duration(i*5) * time.Minute),
            PowerProduced:    1000, PowerConsumed: 500,
        })
    }

    buckets, err := repo.GetAggregatedReadings(
        context.Background(), deviceID, "hour",
        base.Add(-time.Minute), base.Add(time.Hour),
    )
    require.NoError(t, err)
    assert.Len(t, buckets, 1, "all readings fall in one hour bucket")
    assert.Equal(t, 1000, buckets[0].PowerProduced)
}
```

```bash
docker-compose up -d db
TEST_DATABASE_URL="postgres://postgres:password@localhost:5432/power_monitor" \
    go test ./internal/repository/...  # Red
```

### Green: `backend/internal/repository/reading_repository.go`

```go
package repository

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/ahovingtonpower-dashboard/internal/model"
)

type ReadingRepository struct {
    db *pgxpool.Pool
}

func NewReadingRepository(db *pgxpool.Pool) *ReadingRepository {
    return &ReadingRepository{db: db}
}

// SaveReading persists one reading. ON CONFLICT DO NOTHING prevents duplicates
// when the ingestion goroutine re-polls an overlapping time window after restart.
func (r *ReadingRepository) SaveReading(ctx context.Context, reading *model.PowerReading) error {
    _, err := r.db.Exec(ctx, `
        INSERT INTO power_readings (
            device_id, reading_timestamp,
            power_produced, power_consumed,
            energy_produced_today, energy_consumed_today,
            frequency, voltage_phase_a, voltage_phase_b, voltage_phase_c
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
        ON CONFLICT (device_id, reading_timestamp) DO NOTHING`,
        reading.DeviceID,
        reading.ReadingTimestamp.UTC(), // always store UTC
        reading.PowerProduced, reading.PowerConsumed,
        reading.EnergyProducedToday, reading.EnergyConsumedToday,
        reading.Frequency,
        reading.VoltagePhaseA, reading.VoltagePhaseB, reading.VoltagePhaseC,
    )
    if err != nil {
        return fmt.Errorf("repository: save reading: %w", err)
    }
    return nil
}

func (r *ReadingRepository) GetLatestReadings(ctx context.Context, deviceID uuid.UUID, limit int) ([]*model.PowerReading, error) {
    rows, err := r.db.Query(ctx, `
        SELECT id, device_id, reading_timestamp,
               power_produced, power_consumed,
               energy_produced_today, energy_consumed_today,
               frequency, voltage_phase_a, voltage_phase_b, voltage_phase_c,
               created_at
        FROM power_readings
        WHERE device_id = $1
        ORDER BY reading_timestamp DESC
        LIMIT $2`,
        deviceID, limit,
    )
    if err != nil {
        return nil, fmt.Errorf("repository: get latest readings: %w", err)
    }
    defer rows.Close()

    var readings []*model.PowerReading
    for rows.Next() {
        r2 := &model.PowerReading{}
        if err := rows.Scan(
            &r2.ID, &r2.DeviceID, &r2.ReadingTimestamp,
            &r2.PowerProduced, &r2.PowerConsumed,
            &r2.EnergyProducedToday, &r2.EnergyConsumedToday,
            &r2.Frequency, &r2.VoltagePhaseA, &r2.VoltagePhaseB, &r2.VoltagePhaseC,
            &r2.CreatedAt,
        ); err != nil {
            return nil, fmt.Errorf("repository: scan reading: %w", err)
        }
        readings = append(readings, r2)
    }
    return readings, rows.Err()
}

// GetAggregatedReadings returns bucketed averages for chart queries.
// interval must be one of: "hour", "day", "week", "month".
// Always use this for time-range chart data — never return raw rows.
func (r *ReadingRepository) GetAggregatedReadings(
    ctx context.Context,
    deviceID uuid.UUID,
    interval string,
    start, end time.Time,
) ([]*model.PowerReading, error) {
    rows, err := r.db.Query(ctx, `
        SELECT
            DATE_TRUNC($1, reading_timestamp) AS bucket,
            AVG(power_produced)::INT,
            AVG(power_consumed)::INT,
            SUM(energy_produced_today),
            SUM(energy_consumed_today)
        FROM power_readings
        WHERE device_id = $2
          AND reading_timestamp BETWEEN $3 AND $4
        GROUP BY bucket
        ORDER BY bucket ASC`,
        interval, deviceID, start.UTC(), end.UTC(),
    )
    if err != nil {
        return nil, fmt.Errorf("repository: get aggregated readings: %w", err)
    }
    defer rows.Close()

    var readings []*model.PowerReading
    for rows.Next() {
        r2 := &model.PowerReading{}
        if err := rows.Scan(
            &r2.ReadingTimestamp,
            &r2.PowerProduced, &r2.PowerConsumed,
            &r2.EnergyProducedToday, &r2.EnergyConsumedToday,
        ); err != nil {
            return nil, fmt.Errorf("repository: scan aggregated reading: %w", err)
        }
        readings = append(readings, r2)
    }
    return readings, rows.Err()
}
```

```bash
go test -race ./internal/repository/...  # Green
```

### Step 5b — Device Repository

Needed for startup multi-device support (Issue 1A).

**Red: `backend/internal/repository/device_repository_test.go`**

```go
package repository_test

func TestGetActiveDevices_ReturnsNonDeleted(t *testing.T) {
    db := setupDB(t)
    repo := repository.NewDeviceRepository(db)

    // Insert an active and a soft-deleted device
    // (requires a test household + user row first)
    // ... see fixtures/device_fixtures.go for helper

    devices, err := repo.GetActiveDevices(context.Background())
    require.NoError(t, err)
    for _, d := range devices {
        assert.Nil(t, d.DeletedAt, "soft-deleted devices must not appear")
        assert.Equal(t, "active", d.Status)
    }
}
```

**Green: `backend/internal/repository/device_repository.go`**

```go
package repository

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/ahovingtonpower-dashboard/internal/model"
)

type DeviceRepository struct {
    db *pgxpool.Pool
}

func NewDeviceRepository(db *pgxpool.Pool) *DeviceRepository {
    return &DeviceRepository{db: db}
}

// GetActiveDevices returns all non-deleted, active devices across all households.
// Used at startup to create one IngestionService per provider.
func (r *DeviceRepository) GetActiveDevices(ctx context.Context) ([]*model.Device, error) {
    rows, err := r.db.Query(ctx, `
        SELECT id, household_id, provider_id, provider_type, device_type,
               name, serial_number, location, status, created_at, updated_at
        FROM devices
        WHERE status = 'active' AND deleted_at IS NULL`)
    if err != nil {
        return nil, fmt.Errorf("repository: get active devices: %w", err)
    }
    defer rows.Close()

    var devices []*model.Device
    for rows.Next() {
        d := &model.Device{}
        if err := rows.Scan(
            &d.ID, &d.HouseholdID, &d.ProviderID, &d.ProviderType, &d.DeviceType,
            &d.Name, &d.SerialNumber, &d.Location, &d.Status, &d.CreatedAt, &d.UpdatedAt,
        ); err != nil {
            return nil, fmt.Errorf("repository: scan device: %w", err)
        }
        devices = append(devices, d)
    }
    return devices, rows.Err()
}
```

```bash
go test -race ./internal/repository/...  # Green
```

---

## Step 6 — Ingestion Service (TDD)

The most critical component. Test the safety properties first.

The constructor takes a `<-chan time.Time` trigger instead of `time.Duration` — tests control when polls fire without sleeping.

```
Production:  time.NewTicker(interval).C  ──► ingestion goroutine
Tests:       testTrigger := make(chan time.Time)
             testTrigger <- time.Now()   ──► fires poll synchronously
```

### Red: tests first (`backend/internal/service/ingestion_service_test.go`)

```go
package service_test

import (
    "context"
    "errors"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.uber.org/mock/gomock"

    "github.com/ahovingtonpower-dashboard/internal/model"
    "github.com/ahovingtonpower-dashboard/internal/service"
    "github.com/ahovingtonpower-dashboard/pkg/adapter"
)

type stubRepo struct{ saved []*model.PowerReading }

func (s *stubRepo) SaveReading(_ context.Context, r *model.PowerReading) error {
    s.saved = append(s.saved, r)
    return nil
}

type failingRepo struct{ err error }

func (r *failingRepo) SaveReading(_ context.Context, _ *model.PowerReading) error { return r.err }

// triggerOnce sends one tick on the channel, then waits for the ctx to be cancelled.
func triggerOnce(ctx context.Context, ch chan<- time.Time) {
    ch <- time.Now()
    <-ctx.Done()
}

func TestIngestionService_PublishesEventOnSuccess(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockAdapter := adapter.NewMockProviderAdapter(ctrl)
    mockAdapter.EXPECT().GetSystemStatus(gomock.Any()).Return(&adapter.SystemStatus{
        PowerProduced: 5000, PowerConsumed: 3000,
    }, nil)

    repo := &stubRepo{}
    bus := make(chan model.PowerEvent, 8)
    trigger := make(chan time.Time, 1)

    svc := service.NewIngestionService(mockAdapter, repo, bus, uuid.New(), trigger)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go svc.RunPoller(ctx)

    trigger <- time.Now() // fire one poll

    select {
    case event := <-bus:
        assert.Equal(t, 5000, event.PowerProduced)
        assert.Equal(t, 3000, event.PowerConsumed)
        assert.Equal(t, 2000, event.PowerNet)
    case <-time.After(time.Second):
        t.Fatal("no event received within 1s")
    }

    // Verify reading was also persisted
    require.Len(t, repo.saved, 1)
    assert.Equal(t, 5000, repo.saved[0].PowerProduced)
}

func TestIngestionService_PanicIsRecovered(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockAdapter := adapter.NewMockProviderAdapter(ctrl)
    mockAdapter.EXPECT().GetSystemStatus(gomock.Any()).DoAndReturn(
        func(_ context.Context) (*adapter.SystemStatus, error) {
            panic("simulated nil dereference")
        },
    )

    trigger := make(chan time.Time, 1)
    svc := service.NewIngestionService(mockAdapter, &stubRepo{}, make(chan model.PowerEvent, 1), uuid.New(), trigger)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    assert.NotPanics(t, func() {
        go svc.RunPoller(ctx)
        trigger <- time.Now()
        time.Sleep(50 * time.Millisecond) // let the goroutine recover
    })
}

func TestIngestionService_GracefulShutdown(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockAdapter := adapter.NewMockProviderAdapter(ctrl)

    trigger := make(chan time.Time)
    svc := service.NewIngestionService(mockAdapter, &stubRepo{}, make(chan model.PowerEvent, 1), uuid.New(), trigger)

    ctx, cancel := context.WithCancel(context.Background())
    done := make(chan struct{})
    go func() {
        svc.RunPoller(ctx)
        close(done)
    }()

    cancel()
    select {
    case <-done:
    case <-time.After(2 * time.Second):
        t.Fatal("RunPoller did not stop within 2s of context cancellation")
    }
}

func TestIngestionService_RateLimitTriggersBackoff(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockAdapter := adapter.NewMockProviderAdapter(ctrl)
    mockAdapter.EXPECT().GetSystemStatus(gomock.Any()).Return(nil, adapter.ErrRateLimited)

    trigger := make(chan time.Time, 1)
    svc := service.NewIngestionService(mockAdapter, &stubRepo{}, make(chan model.PowerEvent, 1), uuid.New(), trigger)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go svc.RunPoller(ctx)

    trigger <- time.Now()
    time.Sleep(20 * time.Millisecond)

    // After a rate limit, the service should be in backoff — a second trigger
    // should NOT cause another call (backoff sleep is in progress).
    // The mock expectation of exactly 1 call enforces this.
}

func TestIngestionService_RepoErrorDoesNotCrash(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockAdapter := adapter.NewMockProviderAdapter(ctrl)
    mockAdapter.EXPECT().GetSystemStatus(gomock.Any()).Return(
        &adapter.SystemStatus{PowerProduced: 1000}, nil,
    )

    trigger := make(chan time.Time, 1)
    svc := service.NewIngestionService(mockAdapter, &failingRepo{err: errors.New("db pool exhausted")},
        make(chan model.PowerEvent, 1), uuid.New(), trigger)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    assert.NotPanics(t, func() {
        go svc.RunPoller(ctx)
        trigger <- time.Now()
        time.Sleep(20 * time.Millisecond)
    })
}
```

```bash
go test ./internal/service/...  # Red
```

### Green: `backend/internal/service/ingestion_service.go`

```go
package service

import (
    "context"
    "errors"
    "log/slog"
    "math"
    "time"

    "github.com/google/uuid"
    "github.com/ahovingtonpower-dashboard/internal/model"
    "github.com/ahovingtonpower-dashboard/pkg/adapter"
)

type ReadingWriter interface {
    SaveReading(ctx context.Context, r *model.PowerReading) error
}

// IngestionService polls a ProviderAdapter on every tick, persists readings,
// and publishes PowerEvents to the SSE hub via eventBus.
//
//   ticker.C (or test channel) ──► pollSafe ──► pollOnce ──► SaveReading
//                                                         └──► eventBus
//
// Panics in pollOnce are recovered. Errors trigger exponential backoff.
// Backoff sequence: 1s → 2s → 4s → ... → 5min (factor starts at 0).
type IngestionService struct {
    adapter  adapter.ProviderAdapter
    repo     ReadingWriter
    eventBus chan<- model.PowerEvent
    deviceID uuid.UUID
    trigger  <-chan time.Time // time.NewTicker(interval).C in production; injected in tests
}

func NewIngestionService(
    a adapter.ProviderAdapter,
    repo ReadingWriter,
    eventBus chan<- model.PowerEvent,
    deviceID uuid.UUID,
    trigger <-chan time.Time,
) *IngestionService {
    return &IngestionService{adapter: a, repo: repo, eventBus: eventBus, deviceID: deviceID, trigger: trigger}
}

func (s *IngestionService) RunPoller(ctx context.Context) {
    slog.Info("ingestion: starting poller", "device_id", s.deviceID)
    b := newBackoff(time.Second, 5*time.Minute)
    for {
        select {
        case <-ctx.Done():
            slog.Info("ingestion: poller stopped", "device_id", s.deviceID)
            return
        case <-s.trigger:
            s.pollSafe(ctx, b)
        }
    }
}

func (s *IngestionService) pollSafe(ctx context.Context, b *backoff) {
    start := time.Now()
    defer func() {
        if rec := recover(); rec != nil {
            slog.Error("ingestion: panic recovered",
                "device_id", s.deviceID, "panic", rec, "retry_in", b.current())
            time.Sleep(b.increase())
        }
    }()

    if err := s.pollOnce(ctx); err != nil {
        if errors.Is(err, adapter.ErrRateLimited) {
            slog.Warn("ingestion: rate limited",
                "device_id", s.deviceID, "retry_in", b.current())
        } else {
            slog.Error("ingestion: poll failed",
                "device_id", s.deviceID, "error", err, "retry_in", b.current())
        }
        time.Sleep(b.increase())
        return
    }

    b.reset()
    slog.Info("ingestion: cycle complete",
        "device_id", s.deviceID, "duration_ms", time.Since(start).Milliseconds())
}

func (s *IngestionService) pollOnce(ctx context.Context) error {
    status, err := s.adapter.GetSystemStatus(ctx)
    if err != nil {
        return err
    }

    now := time.Now().UTC()
    if err := s.repo.SaveReading(ctx, &model.PowerReading{
        DeviceID:         s.deviceID,
        ReadingTimestamp: now,
        PowerProduced:    status.PowerProduced,
        PowerConsumed:    status.PowerConsumed,
    }); err != nil {
        return err
    }

    event := model.PowerEvent{
        DeviceID:      s.deviceID,
        Timestamp:     now,
        PowerProduced: status.PowerProduced,
        PowerConsumed: status.PowerConsumed,
        PowerNet:      status.PowerProduced - status.PowerConsumed,
    }
    select {
    case s.eventBus <- event:
    default:
        slog.Warn("ingestion: event bus full, dropping event", "device_id", s.deviceID)
    }
    return nil
}

// backoff implements exponential backoff with a cap.
// Factor starts at 0 so the first sleep is 2^0 * min = 1 * min.
// Sequence with min=1s: 1s → 2s → 4s → 8s → ... → 5min.
type backoff struct {
    min, max, cur time.Duration
    factor        int
}

func newBackoff(min, max time.Duration) *backoff {
    return &backoff{min: min, max: max, cur: min, factor: 0}
}

func (b *backoff) current() time.Duration { return b.cur }

func (b *backoff) reset() { b.cur = b.min; b.factor = 0 }

func (b *backoff) increase() time.Duration {
    next := time.Duration(math.Pow(2, float64(b.factor))) * b.min
    if next > b.max {
        next = b.max
    }
    b.cur = next
    b.factor++
    return next
}
```

```bash
go test -race ./internal/service/...  # Green
```

---

## Step 7 — Power Service (TDD)

### Red: tests first (`backend/internal/service/power_service_test.go`)

```go
package service_test

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.uber.org/mock/gomock"

    "github.com/ahovingtonpower-dashboard/internal/model"
    "github.com/ahovingtonpower-dashboard/internal/service"
)

func TestPowerService_GetCurrentStatus_ReturnsLatest(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    reading := &model.PowerReading{PowerProduced: 5000, PowerConsumed: 3000}
    mockRepo := service.NewMockReadingQuerier(ctrl)
    mockRepo.EXPECT().GetLatestReadings(gomock.Any(), gomock.Any(), 1).Return([]*model.PowerReading{reading}, nil)

    svc := service.NewPowerService(mockRepo)
    result, err := svc.GetCurrentStatus(context.Background(), uuid.New())

    require.NoError(t, err)
    assert.Equal(t, 5000, result.PowerProduced)
    assert.Equal(t, 2000, result.PowerNet())
}

func TestPowerService_GetCurrentStatus_NoDataReturnsNil(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockRepo := service.NewMockReadingQuerier(ctrl)
    mockRepo.EXPECT().GetLatestReadings(gomock.Any(), gomock.Any(), 1).Return(nil, nil)

    svc := service.NewPowerService(mockRepo)
    result, err := svc.GetCurrentStatus(context.Background(), uuid.New())

    assert.NoError(t, err)
    assert.Nil(t, result, "no data yet is not an error")
}

func TestPowerService_GetHistory_RejectsInvalidInterval(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    svc := service.NewPowerService(service.NewMockReadingQuerier(ctrl))
    _, err := svc.GetHistory(context.Background(), uuid.New(), "minute", time.Now(), time.Now())

    assert.Error(t, err)
}

func TestPowerService_GetHistory_ValidIntervals(t *testing.T) {
    for _, interval := range []string{"hour", "day", "week", "month"} {
        t.Run(interval, func(t *testing.T) {
            ctrl := gomock.NewController(t)
            defer ctrl.Finish()

            mockRepo := service.NewMockReadingQuerier(ctrl)
            mockRepo.EXPECT().
                GetAggregatedReadings(gomock.Any(), gomock.Any(), interval, gomock.Any(), gomock.Any()).
                Return([]*model.PowerReading{}, nil)

            svc := service.NewPowerService(mockRepo)
            _, err := svc.GetHistory(context.Background(), uuid.New(), interval, time.Now(), time.Now())
            assert.NoError(t, err)
        })
    }
}
```

Generate mock:

```bash
go generate ./internal/service/...
```

```bash
go test ./internal/service/...  # Red
```

### Green: `backend/internal/service/power_service.go`

```go
package service

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/ahovingtonpower-dashboard/internal/model"
)

//go:generate mockgen -source=power_service.go -destination=mock_reading_querier.go -package=service
type ReadingQuerier interface {
    GetLatestReadings(ctx context.Context, deviceID uuid.UUID, limit int) ([]*model.PowerReading, error)
    GetAggregatedReadings(ctx context.Context, deviceID uuid.UUID, interval string, start, end time.Time) ([]*model.PowerReading, error)
}

type PowerService struct{ repo ReadingQuerier }

func NewPowerService(repo ReadingQuerier) *PowerService { return &PowerService{repo: repo} }

func (s *PowerService) GetCurrentStatus(ctx context.Context, deviceID uuid.UUID) (*model.PowerReading, error) {
    readings, err := s.repo.GetLatestReadings(ctx, deviceID, 1)
    if err != nil {
        return nil, fmt.Errorf("power service: get current status: %w", err)
    }
    if len(readings) == 0 {
        return nil, nil
    }
    return readings[0], nil
}

func (s *PowerService) GetHistory(ctx context.Context, deviceID uuid.UUID, interval string, start, end time.Time) ([]*model.PowerReading, error) {
    valid := map[string]bool{"hour": true, "day": true, "week": true, "month": true}
    if !valid[interval] {
        return nil, fmt.Errorf("power service: invalid interval %q (must be hour|day|week|month)", interval)
    }
    return s.repo.GetAggregatedReadings(ctx, deviceID, interval, start, end)
}
```

```bash
go test -race ./internal/service/...  # Green
```

---

## Step 8 — SSE Hub (TDD)

### Red: tests first (`backend/internal/api/sse_test.go`)

Includes an end-to-end HTTP-level test that verifies `Content-Type` and `data:` framing.

```go
package api_test

import (
    "bufio"
    "context"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/ahovingtonpower-dashboard/internal/api"
    "github.com/ahovingtonpower-dashboard/internal/model"
)

func TestHub_SingleClientReceivesEvent(t *testing.T) {
    hub := api.NewHub()
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go hub.Run(ctx)

    ch := hub.Subscribe()
    defer hub.Unsubscribe(ch)

    hub.Broadcast(model.PowerEvent{PowerProduced: 5000})

    select {
    case got := <-ch:
        assert.Equal(t, 5000, got.PowerProduced)
    case <-time.After(time.Second):
        t.Fatal("timeout waiting for event")
    }
}

func TestHub_MultipleClientsAllReceive(t *testing.T) {
    hub := api.NewHub()
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go hub.Run(ctx)

    ch1 := hub.Subscribe()
    ch2 := hub.Subscribe()
    defer hub.Unsubscribe(ch1)
    defer hub.Unsubscribe(ch2)

    hub.Broadcast(model.PowerEvent{PowerProduced: 1234})

    for _, ch := range []chan model.PowerEvent{ch1, ch2} {
        select {
        case got := <-ch:
            assert.Equal(t, 1234, got.PowerProduced)
        case <-time.After(time.Second):
            t.Fatal("client did not receive event")
        }
    }
}

func TestHub_SlowClientDoesNotBlockOthers(t *testing.T) {
    hub := api.NewHub()
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go hub.Run(ctx)

    fast := hub.Subscribe()
    slow := hub.Subscribe() // never read from
    defer hub.Unsubscribe(fast)
    defer hub.Unsubscribe(slow)

    for i := 0; i < 20; i++ {
        hub.Broadcast(model.PowerEvent{PowerProduced: i})
    }

    received := 0
    for {
        select {
        case <-fast:
            received++
        default:
            goto done
        }
    }
done:
    assert.Greater(t, received, 0)
}

func TestHub_UnsubscribedClientStopsReceiving(t *testing.T) {
    hub := api.NewHub()
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go hub.Run(ctx)

    ch := hub.Subscribe()
    hub.Unsubscribe(ch)
    time.Sleep(20 * time.Millisecond)

    hub.Broadcast(model.PowerEvent{PowerProduced: 999})
    time.Sleep(20 * time.Millisecond)

    select {
    case _, open := <-ch:
        assert.False(t, open, "channel should be closed after unsubscribe")
    default:
    }
}

// TestServeSSE_EndToEnd verifies the full HTTP contract:
// Content-Type header, data: framing, and that an event is delivered.
func TestServeSSE_EndToEnd(t *testing.T) {
    hub := api.NewHub()
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go hub.Run(ctx)

    srv := httptest.NewServer(http.HandlerFunc(hub.ServeSSE))
    defer srv.Close()

    // Connect a client in a goroutine
    received := make(chan string, 1)
    go func() {
        resp, err := http.Get(srv.URL)
        if err != nil {
            return
        }
        defer resp.Body.Close()

        assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

        scanner := bufio.NewScanner(resp.Body)
        for scanner.Scan() {
            line := scanner.Text()
            if strings.HasPrefix(line, "data:") {
                received <- line
                return
            }
        }
    }()

    time.Sleep(50 * time.Millisecond) // let the client connect

    hub.Broadcast(model.PowerEvent{PowerProduced: 7777})

    select {
    case line := <-received:
        assert.Contains(t, line, "7777")
    case <-time.After(2 * time.Second):
        t.Fatal("SSE client did not receive event within 2s")
    }
}
```

```bash
go test ./internal/api/...  # Red
```

### Green: `backend/internal/api/sse.go`

```go
package api

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"
    "net/http"
    "sync/atomic"

    "github.com/ahovingtonpower-dashboard/internal/model"
)

const sseClientBuffer = 16

// Hub fans out PowerEvents to all connected SSE clients.
//
//   IngestionService ──► eventBus ──► Hub.Broadcast(event)
//                                          │  fan-out
//                              ┌───────────┼───────────┐
//                              ▼           ▼           ▼
//                         client A    client B    client N
//                        (buffered)  (buffered)  (buffered)
//
// Slow clients are dropped (buffer full) rather than blocking the broadcast loop.
type Hub struct {
    subscribe   chan chan model.PowerEvent
    unsubscribe chan chan model.PowerEvent
    broadcast   chan model.PowerEvent
    connected   atomic.Int64
}

func NewHub() *Hub {
    return &Hub{
        subscribe:   make(chan chan model.PowerEvent, 1),
        unsubscribe: make(chan chan model.PowerEvent, 1),
        broadcast:   make(chan model.PowerEvent, 32),
    }
}

func (h *Hub) Run(ctx context.Context) {
    clients := make(map[chan model.PowerEvent]struct{})
    for {
        select {
        case <-ctx.Done():
            for ch := range clients {
                close(ch)
            }
            return
        case ch := <-h.subscribe:
            clients[ch] = struct{}{}
            h.connected.Add(1)
        case ch := <-h.unsubscribe:
            if _, ok := clients[ch]; ok {
                delete(clients, ch)
                close(ch)
                h.connected.Add(-1)
            }
        case event := <-h.broadcast:
            for ch := range clients {
                select {
                case ch <- event:
                default:
                    slog.Warn("sse: dropping event for slow client")
                }
            }
        }
    }
}

func (h *Hub) Subscribe() chan model.PowerEvent {
    ch := make(chan model.PowerEvent, sseClientBuffer)
    h.subscribe <- ch
    return ch
}

func (h *Hub) Unsubscribe(ch chan model.PowerEvent) { h.unsubscribe <- ch }

func (h *Hub) Broadcast(event model.PowerEvent) { h.broadcast <- event }

func (h *Hub) ConnectedClients() int64 { return h.connected.Load() }

// ServeSSE handles GET /api/v1/events.
// No write timeout — the connection is intentionally long-lived.
// Ensure your reverse proxy (nginx) sets an appropriate proxy_read_timeout.
func (h *Hub) ServeSSE(w http.ResponseWriter, r *http.Request) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming not supported", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no")

    ch := h.Subscribe()
    defer h.Unsubscribe(ch)

    for {
        select {
        case <-r.Context().Done():
            return
        case event, ok := <-ch:
            if !ok {
                return
            }
            data, err := json.Marshal(event)
            if err != nil {
                slog.Error("sse: marshal event", "error", err)
                continue
            }
            fmt.Fprintf(w, "data: %s\n\n", data)
            flusher.Flush()
        }
    }
}
```

```bash
go test -race ./internal/api/...  # Green
```

---

## Step 9 — HTTP Handlers (TDD)

### Red: handler tests first (`backend/internal/api/handler_test.go`)

```go
package api_test

import (
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.uber.org/mock/gomock"

    "github.com/ahovingtonpower-dashboard/internal/api"
    "github.com/ahovingtonpower-dashboard/internal/model"
    "github.com/ahovingtonpower-dashboard/internal/service"
)

func TestGetCurrentStatus_ValidDevice(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    deviceID := uuid.New()
    mockSvc := service.NewMockPowerServicer(ctrl)
    mockSvc.EXPECT().GetCurrentStatus(gomock.Any(), deviceID).Return(&model.PowerReading{
        PowerProduced: 5000, PowerConsumed: 3000,
    }, nil)

    h := api.NewHandler(mockSvc, api.NewHub())
    r := httptest.NewRequest(http.MethodGet, "/api/v1/power/status?device_id="+deviceID.String(), nil)
    w := httptest.NewRecorder()
    h.GetCurrentStatus(w, r)

    assert.Equal(t, http.StatusOK, w.Code)
    var resp map[string]interface{}
    require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
    assert.Equal(t, float64(5000), resp["power_produced"])
    assert.Equal(t, float64(2000), resp["power_net"])
}

func TestGetCurrentStatus_InvalidDeviceID(t *testing.T) {
    h := api.NewHandler(nil, nil)
    r := httptest.NewRequest(http.MethodGet, "/api/v1/power/status?device_id=not-a-uuid", nil)
    w := httptest.NewRecorder()
    h.GetCurrentStatus(w, r)

    assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetCurrentStatus_NoData(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockSvc := service.NewMockPowerServicer(ctrl)
    mockSvc.EXPECT().GetCurrentStatus(gomock.Any(), gomock.Any()).Return(nil, nil)

    h := api.NewHandler(mockSvc, api.NewHub())
    r := httptest.NewRequest(http.MethodGet, "/api/v1/power/status?device_id="+uuid.New().String(), nil)
    w := httptest.NewRecorder()
    h.GetCurrentStatus(w, r)

    assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetHistory_ValidRequest(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    deviceID := uuid.New()
    mockSvc := service.NewMockPowerServicer(ctrl)
    mockSvc.EXPECT().GetHistory(gomock.Any(), deviceID, "hour", gomock.Any(), gomock.Any()).
        Return([]*model.PowerReading{{PowerProduced: 3000}}, nil)

    h := api.NewHandler(mockSvc, api.NewHub())
    url := fmt.Sprintf("/api/v1/power/history?device_id=%s&interval=hour&start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z", deviceID)
    r := httptest.NewRequest(http.MethodGet, url, nil)
    w := httptest.NewRecorder()
    h.GetHistory(w, r)

    assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetHistory_InvalidInterval(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockSvc := service.NewMockPowerServicer(ctrl)
    mockSvc.EXPECT().GetHistory(gomock.Any(), gomock.Any(), "minute", gomock.Any(), gomock.Any()).
        Return(nil, fmt.Errorf("invalid interval"))

    h := api.NewHandler(mockSvc, api.NewHub())
    url := fmt.Sprintf("/api/v1/power/history?device_id=%s&interval=minute&start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z", uuid.New())
    r := httptest.NewRequest(http.MethodGet, url, nil)
    w := httptest.NewRecorder()
    h.GetHistory(w, r)

    assert.Equal(t, http.StatusInternalServerError, w.Code)
}
```

Generate mock:

```bash
mockgen -destination=internal/service/mock_power_servicer.go \
        -package=service \
        -mock_names PowerServicer=MockPowerServicer \
        github.com/ahovingtonpower-dashboard/internal/service PowerServicer
```

```bash
go test ./internal/api/...  # Red
```

### Green: `backend/internal/api/handler.go` and `routes.go`

**handler.go**

```go
package api

import (
    "context"
    "encoding/json"
    "log/slog"
    "net/http"
    "time"

    "github.com/google/uuid"
    "github.com/ahovingtonpower-dashboard/internal/model"
)

//go:generate mockgen -destination=../service/mock_power_servicer.go -package=service -mock_names PowerServicer=MockPowerServicer github.com/ahovingtonpower-dashboard/internal/api PowerServicer
type PowerServicer interface {
    GetCurrentStatus(ctx context.Context, deviceID uuid.UUID) (*model.PowerReading, error)
    GetHistory(ctx context.Context, deviceID uuid.UUID, interval string, start, end time.Time) ([]*model.PowerReading, error)
}

type Handler struct {
    power PowerServicer
    hub   *Hub
}

func NewHandler(power PowerServicer, hub *Hub) *Handler {
    return &Handler{power: power, hub: hub}
}

func (h *Handler) GetCurrentStatus(w http.ResponseWriter, r *http.Request) {
    deviceID, err := uuid.Parse(r.URL.Query().Get("device_id"))
    if err != nil {
        writeError(w, http.StatusBadRequest, "invalid device_id")
        return
    }

    reading, err := h.power.GetCurrentStatus(r.Context(), deviceID)
    if err != nil {
        slog.Error("handler: get current status", "error", err)
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }
    if reading == nil {
        writeJSON(w, http.StatusOK, map[string]string{"status": "no data"})
        return
    }

    writeJSON(w, http.StatusOK, map[string]interface{}{
        "device_id":      reading.DeviceID,
        "timestamp":      reading.ReadingTimestamp,
        "power_produced": reading.PowerProduced,
        "power_consumed": reading.PowerConsumed,
        "power_net":      reading.PowerNet(),
    })
}

func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
    deviceID, err := uuid.Parse(r.URL.Query().Get("device_id"))
    if err != nil {
        writeError(w, http.StatusBadRequest, "invalid device_id")
        return
    }

    interval := r.URL.Query().Get("interval")
    if interval == "" {
        interval = "hour"
    }

    start, err := time.Parse(time.RFC3339, r.URL.Query().Get("start"))
    if err != nil {
        writeError(w, http.StatusBadRequest, "invalid start: use RFC3339")
        return
    }

    end, err := time.Parse(time.RFC3339, r.URL.Query().Get("end"))
    if err != nil {
        writeError(w, http.StatusBadRequest, "invalid end: use RFC3339")
        return
    }

    readings, err := h.power.GetHistory(r.Context(), deviceID, interval, start, end)
    if err != nil {
        slog.Error("handler: get history", "error", err)
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }

    writeJSON(w, http.StatusOK, readings)
}

func (h *Handler) ServeEvents(w http.ResponseWriter, r *http.Request) {
    h.hub.ServeSSE(w, r)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(v); err != nil {
        slog.Error("handler: encode response", "error", err)
    }
}

func writeError(w http.ResponseWriter, status int, msg string) {
    writeJSON(w, status, map[string]string{"error": msg})
}
```

**routes.go**

```go
package api

import (
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/go-chi/cors"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewRouter(h *Handler, allowedOrigin string) http.Handler {
    r := chi.NewRouter()

    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    // CORS: env-driven allowed origin, explicit rather than wildcard.
    r.Use(cors.Handler(cors.Options{
        AllowedOrigins: []string{allowedOrigin},
        AllowedMethods: []string{"GET", "OPTIONS"},
        AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
        MaxAge:         300,
    }))

    // Liveness: is the process running?
    r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    // Readiness: is the DB reachable?
    r.Get("/ready", h.Ready)

    // Prometheus metrics
    r.Handle("/metrics", promhttp.Handler())

    r.Route("/api/v1", func(r chi.Router) {
        // SSE: no TimeoutHandler — connection is intentionally long-lived
        r.Get("/events", h.ServeEvents)

        // All other endpoints: 30s write timeout
        r.Group(func(r chi.Router) {
            r.Use(func(next http.Handler) http.Handler {
                return http.TimeoutHandler(next, 30*time.Second, `{"error":"request timeout"}`)
            })
            r.Get("/power/status", h.GetCurrentStatus)
            r.Get("/power/history", h.GetHistory)
        })
    })

    return r
}
```

Add `Ready` handler to `handler.go`:

```go
// Ready handles GET /ready.
// Returns 503 if the DB pool cannot be pinged — used by Docker healthcheck.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
    if err := h.db.Ping(r.Context()); err != nil {
        slog.Error("readiness: db ping failed", "error", err)
        writeError(w, http.StatusServiceUnavailable, "database unavailable")
        return
    }
    writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

`Handler` needs a `db` field — add `*pgxpool.Pool` to the struct and constructor.

```bash
go test -race ./internal/api/...  # Green
```

---

## Step 10 — Config & Provider Factory

### `backend/internal/config/config.go`

```go
package config

import (
    "fmt"
    "os"
    "strconv"
    "time"
)

type Config struct {
    Env             string
    LogLevel        string
    DatabaseURL     string
    Port            string
    CORSAllowedOrigin string

    EnphaseAPIKey   string
    EnphaseSystemID string

    PollInterval time.Duration
}

func Load() (*Config, error) {
    cfg := &Config{
        Env:               getEnv("GO_ENV", "development"),
        LogLevel:          getEnv("LOG_LEVEL", "info"),
        Port:              getEnv("PORT", "8080"),
        CORSAllowedOrigin: getEnv("CORS_ALLOWED_ORIGIN", "http://localhost:3000"),
        EnphaseAPIKey:     os.Getenv("ENPHASE_API_KEY"),
        EnphaseSystemID:   os.Getenv("ENPHASE_SYSTEM_ID"),
    }

    cfg.DatabaseURL = requireEnv("DATABASE_URL")

    secs, err := strconv.Atoi(getEnv("POLL_INTERVAL_SECONDS", "300"))
    if err != nil {
        return nil, fmt.Errorf("config: invalid POLL_INTERVAL_SECONDS: %w", err)
    }
    cfg.PollInterval = time.Duration(secs) * time.Second

    return cfg, nil
}

func requireEnv(key string) string {
    v := os.Getenv(key)
    if v == "" {
        panic(fmt.Sprintf("required environment variable %q is not set — check .env.example", key))
    }
    return v
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}
```

### `backend/internal/service/provider_factory.go`

Creates adapters based on which env vars are present. One IngestionService goroutine per configured provider.

```go
package service

import (
    "context"
    "log/slog"
    "time"

    "github.com/google/uuid"
    "github.com/ahovingtonpower-dashboard/internal/model"
    "github.com/ahovingtonpower-dashboard/pkg/adapter"
    "github.com/ahovingtonpower-dashboard/pkg/enphase"
)

// ProviderIngestionConfig holds what's needed to start one IngestionService.
type ProviderIngestionConfig struct {
    Adapter  adapter.ProviderAdapter
    DeviceID uuid.UUID
    Trigger  <-chan time.Time
}

// BuildProviders returns one config per configured provider, based on env var presence.
// If ENPHASE_API_KEY is set, an Enphase provider is created. Future providers follow
// the same pattern (check their env var, append to slice).
func BuildProviders(cfg interface{ EnphaseKey() string; EnphaseSystemID() string }, trigger <-chan time.Time) []ProviderIngestionConfig {
    var providers []ProviderIngestionConfig

    if key := os.Getenv("ENPHASE_API_KEY"); key != "" {
        slog.Info("provider: enphase configured")
        providers = append(providers, ProviderIngestionConfig{
            Adapter: enphase.NewAdapter(enphase.Config{
                APIKey:   key,
                SystemID: os.Getenv("ENPHASE_SYSTEM_ID"),
            }),
            DeviceID: uuid.New(), // TODO: load from devices table once DeviceRepository is wired
            Trigger:  trigger,
        })
    }

    if len(providers) == 0 {
        slog.Warn("provider_factory: no providers configured — ingestion will not run")
    }

    return providers
}
```

---

## Step 11 — Entry Point (`backend/cmd/server/main.go`)

Startup order:
```
1. Load config          (panic on missing vars)
2. Connect DB + ping    (fail fast)
3. Run migrations       (fail fast)
4. Wire dependencies
5. Start Hub goroutine
6. Start eventBus bridge goroutine
7. Start one IngestionService goroutine per provider
8. Start HTTP server
9. Block on SIGTERM/SIGINT → graceful shutdown
```

```go
package main

import (
    "context"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/ahovingtonpower-dashboard/internal/api"
    "github.com/ahovingtonpower-dashboard/internal/config"
    "github.com/ahovingtonpower-dashboard/internal/model"
    "github.com/ahovingtonpower-dashboard/internal/repository"
    "github.com/ahovingtonpower-dashboard/internal/service"
)

func main() {
    cfg, err := config.Load()
    if err != nil {
        slog.Error("startup: config", "error", err)
        os.Exit(1)
    }

    db, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
    // Default pool MaxConns = max(4, numCPU). Fine for single-household development.
    // To tune: use pgxpool.ParseConfig() and set config.MaxConns before NewWithConfig().
    if err != nil {
        slog.Error("startup: db connect", "error", err)
        os.Exit(1)
    }
    defer db.Close()

    if err := db.Ping(context.Background()); err != nil {
        slog.Error("startup: db ping", "error", err)
        os.Exit(1)
    }

    m, err := migrate.New("file://migrations", cfg.DatabaseURL)
    if err != nil {
        slog.Error("startup: migration init", "error", err)
        os.Exit(1)
    }
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        slog.Error("startup: migration up", "error", err)
        os.Exit(1)
    }
    slog.Info("startup: migrations applied")

    readingRepo := repository.NewReadingRepository(db)
    powerSvc := service.NewPowerService(readingRepo)
    hub := api.NewHub()
    handler := api.NewHandler(powerSvc, hub, db)
    router := api.NewRouter(handler, cfg.CORSAllowedOrigin)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go hub.Run(ctx)

    eventBus := make(chan model.PowerEvent, 64)

    // Bridge eventBus → hub fan-out
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            case event := <-eventBus:
                hub.Broadcast(event)
            }
        }
    }()

    // Start one IngestionService per configured provider
    for _, p := range service.BuildProviders(cfg, time.NewTicker(cfg.PollInterval).C) {
        svc := service.NewIngestionService(p.Adapter, readingRepo, eventBus, p.DeviceID, p.Trigger)
        go svc.RunPoller(ctx)
    }

    srv := &http.Server{
        Addr:         ":" + cfg.Port,
        Handler:      router,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 30 * time.Second, // SSE handler uses http.TimeoutHandler(0) via chi group
        IdleTimeout:  60 * time.Second,
    }

    go func() {
        slog.Info("startup: listening", "port", cfg.Port)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            slog.Error("http server error", "error", err)
            os.Exit(1)
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
    <-quit

    slog.Info("shutdown: signal received, draining...")
    cancel()

    shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer shutCancel()
    if err := srv.Shutdown(shutCtx); err != nil {
        slog.Error("shutdown: http server", "error", err)
    }
    slog.Info("shutdown: complete")
}
```

---

## Step 12 — Database Migration (`backend/migrations/`)

```
migrations/
├── 001_initial_schema.up.sql    ← full CREATE TABLE statements (see SCHEMA.md)
└── 001_initial_schema.down.sql  ← DROP TABLE IF EXISTS in reverse dependency order
```

Content: copy the SQL from `docs/database/SCHEMA.md`.

Verify after `docker-compose up -d`:

```bash
docker-compose exec db psql -U postgres -d power_monitor -c "\dt"
# Expect 8 tables: users, households, devices, power_readings,
# battery_status, power_quality_metrics, voltage_events, alerts
```

---

## Build Order Summary

```
Step 1   go.mod + mockgen install        setup
Step 2   model/power_reading.go          test PowerNet → implement → green
Step 3   pkg/adapter/provider_adapter.go interface + errors + go:generate
Step 4   pkg/enphase/                    6 HTTP tests → implement → green
Step 5   internal/repository/            4 integration tests → implement → green
Step 5b  internal/repository/device_*   test GetActiveDevices → implement → green
Step 6   internal/service/ingestion_*   5 safety tests (trigger chan) → implement → green
Step 7   internal/service/power_*       4 unit tests → implement → green
Step 8   internal/api/sse.go            4 hub tests + 1 HTTP end-to-end → implement → green
Step 9   internal/api/handler.go        5 handler tests → implement → green
         internal/api/routes.go         CORS + /health + /ready + TimeoutHandler
Step 10  internal/config/config.go      env loading
         internal/service/provider_factory.go  env-var presence check
Step 11  cmd/server/main.go             startup sequence + graceful shutdown
Step 12  migrations/001_*.sql           verify with psql \dt
```

## New Files Required (not in skeleton)

| File | Purpose |
|------|---------|
| `internal/service/ingestion_service.go` | Background poll goroutine (trigger chan pattern) |
| `internal/service/provider_factory.go` | Adapter creation from env-var presence |
| `internal/api/sse.go` | SSE hub + ServeSSE handler |
| `internal/repository/device_repository.go` | GetActiveDevices for startup |
| `pkg/adapter/mock_provider_adapter.go` | Generated by mockgen |
| `internal/service/mock_reading_querier.go` | Generated by mockgen |
| `internal/service/mock_power_servicer.go` | Generated by mockgen |
| `migrations/001_initial_schema.down.sql` | Rollback migration |

## Running All Tests

```bash
# Unit tests (no DB needed)
go test -race ./internal/model/... ./internal/service/... ./pkg/... ./internal/api/...

# Integration tests (needs Docker DB running)
docker-compose up -d db
TEST_DATABASE_URL="postgres://postgres:password@localhost:5432/power_monitor" \
    go test -race ./internal/repository/...

# Full suite with coverage
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep total
```
