# TODOS

Deferred work, vision items, and known gaps identified during plan review.
Format: Priority (P1/P2/P3), Effort (S/M/L/XL), Status.

---

## Architecture

### [P1/M] Add SSE hub for real-time frontend updates
**What:** Replace REST polling with Server-Sent Events. Backend maintains a fan-out hub; ingestion goroutine pushes `PowerEvent` onto an internal channel; hub broadcasts to all connected clients.
**Why:** The docs promise "real-time updates" but the current REST-only design introduces visible lag and wastes provider API quota. SSE is HTTP-native, handles reconnects automatically, and Go's `http.Flusher` interface makes implementation straightforward.
**How to apply:** Add `GET /api/v1/events` SSE endpoint. Internal `EventHub` struct with `Subscribe()/Unsubscribe()` and a fan-out goroutine. Buffered channels per client prevent slow clients from blocking the hub.
**Depends on:** Background ingestion goroutine (below)

---

### [P1/M] Background ingestion goroutine with recover() + exponential backoff
**What:** A goroutine started at server init that polls each registered provider adapter on a configurable interval (default: 5 min). Wraps the poll body in `recover()` to catch panics from malformed API responses. Uses exponential backoff (1s → 2s → 4s → ... cap 5min) on consecutive errors.
**Why:** The current docs show a `Poll()` snippet but never explain who calls it, how it's supervised, or what happens when it panics. A nil pointer from a bad API response will crash the backend silently.
**How to apply:** Implement in `internal/service/ingestion_service.go`. Accept a `context.Context` for graceful shutdown. Log every cycle with structured fields: provider, device_id, readings_persisted, duration_ms, next_poll_in.
**Depends on:** Nothing — this is foundational

---

### [P1/S] Add users + households tables to initial schema migration
**What:** Add `users` (id, email, password_hash, created_at) and `households` (id, user_id, name, timezone, created_at) tables. Add `household_id` FK to `devices` and `api_credentials`.
**Why:** Without user/household dimension in the schema from day 1, adding authentication later requires a full table rebuild with backfill migrations. The schema is a one-way door.
**How to apply:** Add to `backend/migrations/001_initial_schema.sql` before any data exists. All device queries gain a `WHERE household_id = $1` clause.
**Depends on:** Nothing

---

### [P1/S] Use TIMESTAMPTZ everywhere (not TIMESTAMP)
**What:** Change all `TIMESTAMP` columns in the schema to `TIMESTAMPTZ` (timestamp with time zone).
**Why:** In regions with DST (e.g. Australia AEST/AEDT), queries spanning daylight saving transitions return incorrect data for billing calculations when using naive timestamps. This is a data correctness bug, not a preference.
**How to apply:** Change all 8 tables in `001_initial_schema.sql`. Store all timestamps as UTC at the application layer. Display in local timezone at the frontend layer.
**Depends on:** Nothing — must be done before any data is written

---

### [P1/S] Add UNIQUE(device_id, reading_timestamp) to power_readings
**What:** Add a unique constraint on `(device_id, reading_timestamp)` to `power_readings`, `battery_status`, and `power_quality_metrics`. Use `INSERT ... ON CONFLICT DO NOTHING` in the repository.
**Why:** The polling loop will re-read overlapping time windows on restart. Without this constraint, duplicate rows accumulate silently and corrupt aggregate calculations.
**How to apply:** Add `UNIQUE(device_id, reading_timestamp)` to migration 001. Repository `Save()` methods use `ON CONFLICT DO NOTHING`.
**Depends on:** Nothing

---

### [P1/S] Remove api_credentials table; use env vars only
**What:** Drop the `api_credentials` table from the schema. Provider API keys are stored exclusively in environment variables / Docker secrets.
**Why:** The table was documented as storing `encrypted_value TEXT` but the encryption key management was undefined — creating circular security (key stored in env, encrypts a value stored in DB that could just be the env var). For a single-household app, env vars are the correct primitive.
**How to apply:** Remove from `001_initial_schema.sql`. Remove from `SCHEMA.md`. Update `config.go` to load provider keys from env. Remove from `api/INTEGRATION.md`.
**Depends on:** Nothing

---

### [P1/S] Add /api/v1 prefix to all routes
**What:** All backend API routes use `/api/v1/` prefix.
**Why:** Adding versioning after clients are built requires breaking changes. This is a one-way door.
**How to apply:** Set as the Chi router mount prefix in `routes.go`. Update frontend `api.ts` base URL.
**Depends on:** Nothing

---

### [P1/S] Add .env.example file
**What:** Create `.env.example` with all required environment variables, documented, with placeholder values.
**Why:** The DEVELOPMENT.md says `cp .env.example .env` but no `.env.example` exists. A new developer will not know what variables are required.
**How to apply:** Create at repo root with all variables from DEVELOPMENT.md and config.go.
**Depends on:** Nothing

---

### [P1/M] Use golang-migrate for database migrations
**What:** Add `golang-migrate/migrate` library. Migration runner in `cmd/server/main.go` applies unapplied migrations from `backend/migrations/`. Tracks state in `schema_migrations` table.
**Why:** The current plan references `go run cmd/server/main.go migrate` but `main.go` is a stub with no migration logic. A hand-rolled runner risks re-running migrations, skipping failed ones, or corrupting state.
**How to apply:** Add `github.com/golang-migrate/migrate/v4` to `go.mod`. Call `migrate.Up()` at server startup (fail fast if migrations fail). Number migration files sequentially: `001_initial_schema.up.sql`, `001_initial_schema.down.sql`.
**Depends on:** Nothing

---

## Code Quality

### [P1/S] Fix type assertion panic in CreateAdapter
**What:** Replace `cfg := config.(map[string]string)` with a safe two-value assertion and explicit error return.
**Why:** The current code in `INTEGRATION.md` will panic at runtime if the config type is wrong. The Go compiler cannot catch this. Panics in service code crash the goroutine.
**How to apply:**
```go
cfg, ok := config.(map[string]string)
if !ok {
    return nil, fmt.Errorf("invalid config type for provider %q: got %T, want map[string]string", providerType, config)
}
```
**Depends on:** Nothing

---

### [P1/S] Check HTTP status codes in all adapter HTTP calls
**What:** After every `httpClient.Do(req)`, check `resp.StatusCode` before decoding. Return typed sentinel errors for 401 (`ErrAuthExpired`), 429 (`ErrRateLimited`), 5xx (`ErrProviderUnavailable`).
**Why:** The current `GetSystemStatus` example in `INTEGRATION.md` decodes the body regardless of status code. A 429 or 503 silently decodes as an empty struct, which then persists zeroed readings to the database.
**How to apply:** Define error sentinel types in `pkg/adapter/errors.go`. Each adapter method checks status before decode.
**Depends on:** Nothing

---

### [P1/S] Use typed credentials, not interface{}
**What:** Replace `Authenticate(ctx context.Context, credentials interface{}) error` with typed per-provider config structs, or a `ProviderConfig` interface.
**Why:** `interface{}` in the adapter interface requires type assertions at every call site, which panic on type mismatch. The Go compiler provides zero safety.
**How to apply:** Define `type EnphaseConfig struct { APIKey, SystemID string }`. Either use a `ProviderConfig` marker interface or make each adapter's constructor accept its own typed config (preferred — simpler, no interface needed).
**Depends on:** Remove api_credentials table (above)

---

### [P1/S] Remove derived column power_net from power_readings
**What:** Drop `power_net INT` from `power_readings`. Compute it on read: `power_produced - power_consumed`.
**Why:** Storing a derived value creates a consistency risk — if either source column is corrected, `power_net` becomes stale and corrupts historical analysis.
**How to apply:** Remove from migration 001. Add a computed column or view if needed. Update repository queries to compute inline.
**Depends on:** Nothing

---

### [P1/S] Remove duplicate battery state columns
**What:** Drop either `charge_percentage NUMERIC(5,2)` or `state_of_charge INT` from `battery_status`. They represent the same value at different precision.
**Why:** Duplicate columns with different types will drift. Keep `charge_percentage NUMERIC(5,2)` as it has better precision.
**Depends on:** Nothing

---

### [P2/M] Replace system_status table with a materialized view or computed endpoint
**What:** Drop the `system_status` table. Compute system-wide totals in a service method that aggregates the latest row per device from `power_readings` and `battery_status`.
**Why:** `system_status` duplicates data that already exists in other tables. Any lag in the write pipeline causes the dashboard to show stale totals while device tables are current.
**Depends on:** Background ingestion goroutine

---

## Testing

### [P1/M] Replace real-HTTP adapter tests with httptest.Server mocks
**What:** All adapter unit tests use `httptest.NewServer()` to serve mock responses instead of calling real provider URLs.
**Why:** The current test in `TESTING.md` calls `adapter.GetSystemStatus(context.Background())` with a test key against a real URL — this always fails in CI and reveals API keys in test output.
**How to apply:** Each adapter test creates a `httptest.NewServer(http.HandlerFunc(...))`, sets `adapter.baseURL = testServer.URL`. Tests cover: 200 valid JSON, 200 invalid JSON, 429, 5xx, network timeout.
**Depends on:** Nothing

---

### [P2/M] Add SSE hub unit tests
**What:** Test the SSE fan-out hub: single client receives events, multiple clients all receive the same event, slow client (full buffer) does not block other clients, client disconnect triggers cleanup.
**Why:** The hub is shared mutable state accessed by concurrent goroutines. Without tests, race conditions will surface unpredictably in production.
**How to apply:** Use `go test -race` flag. Test with `httptest.NewRecorder()`.
**Depends on:** SSE hub implementation

---

### [P2/S] Add -race flag to CI test command
**What:** Add `go test -race ./...` to GitHub Actions CI alongside the standard `go test ./...`.
**Why:** The ingestion goroutine, SSE hub, and shared state are concurrent. Race conditions that don't manifest in normal tests often appear with `-race`.
**Depends on:** Nothing

---

## Observability

### [P2/M] Add structured log fields for ingestion cycles
**What:** Every ingestion cycle logs: provider, device_id, readings_persisted, duration_ms, next_poll_in. Every error logs: provider, error, retry_in.
**Why:** Without consistent structured fields, logs are unsearchable. The first time something goes wrong at 3am, you want to be able to run `grep '"provider":"enphase"' app.log | grep '"level":"ERROR"'`.
**Depends on:** Background ingestion goroutine

---

### [P2/M] Add /metrics Prometheus endpoint
**What:** Expose `GET /metrics` using `promhttp.Handler()`. Track: `ingestion_cycle_duration_ms`, `provider_api_errors_total{provider,status_code}`, `power_readings_persisted_total`, `sse_connected_clients`.
**Why:** For EXPANSION mode, these metrics are the difference between "I think it's working" and "I know it's working." Pairs with Grafana for a complete observability picture.
**Depends on:** Background ingestion goroutine, SSE hub

---

### [P2/S] Periodic refresh for `usePowerHistory`
**What:** Add an optional `refreshInterval` to `usePowerHistory` that re-fetches history data on a timer (e.g. every 5 minutes for the 'hour' view).
**Why:** History data goes stale during long sessions. SSE covers the live metric cards, but the area chart won't update unless the user manually changes intervals. For the 'hour' view this is immediately visible — the chart's right edge stays frozen.
**How to apply:** Add `useEffect` with `setInterval` (the global one) in `usePowerHistory`. Only activate when `interval === 'hour'` since longer views (day/week) change slowly. Pair with the existing `loading` state so the chart shows a subtle refresh indicator.
**Depends on:** Nothing

---

### [P2/M] Wire BatteryCard once backend exposes battery data
**What:** Add `battery_charge_pct`, `battery_power_w`, and `battery_direction` fields to `PowerStatus` and `PowerEvent` API responses. Implement `BatteryCard` component in the frontend (plan exists in IMPLEMENTATION.md Step 8 — removed from MVP but preserved as a template). Wire `GetBatteryStatus()` in the Enphase adapter (currently returns `nil, nil`).
**Why:** Battery state (charge %, charging vs discharging, wattage) is core to the "home energy system" dashboard concept. It was removed from MVP only because the backend returns no data yet, not because it's unimportant.
**How to apply:** 1. Extend `PowerReading` model and DB schema with battery columns. 2. Implement `GetBatteryStatus()` in Enphase adapter using the `/ivp/ensemble/inventory` endpoint. 3. Merge battery state into `PowerStatus` API response. 4. Restore `BatteryCard` component from IMPLEMENTATION.md Step 8 template.
**Depends on:** Enphase battery API endpoint research; `GetBatteryStatus()` currently returns `nil, nil`

---

## Future Vision (P3)

### [P3/L] Energy cost & savings tracking
**What:** Store electricity tariff rates (flat or time-of-use schedules). Calculate daily $ saved (solar generation × tariff rate), $ imported (grid consumption × tariff), $ exported (feed-in tariff). Display "Today's savings" card on dashboard.
**Why:** Homeowners want to know the dollar value of their system, not just watts. This is the most-requested feature in every solar monitoring community.
**Depends on:** Core monitoring pipeline working, users/households table

---

### [P3/M] Anomaly detection for panel/inverter underperformance
**What:** Compare rolling 7-day average output per device vs current output. Alert if a device produces <80% of its expected output (weather-adjusted). Surface as an alert in the dashboard.
**Why:** Catches real faults — shading from new obstructions, inverter degradation, panel damage. Current apps from providers don't do this.
**Depends on:** 7+ days of baseline data, alerts pipeline working

---

### [P3/L] Weather correlation chart
**What:** Overlay cloud cover / solar irradiance data from a weather API (Open-Meteo is free, no key required) on the solar generation chart.
**Why:** Explains generation dips naturally. Users stop wondering "why was Sunday bad?" and see "ah, 80% cloud cover."
**Depends on:** Historical charting feature, cost is per API call

---

### [P3/S] Grid outage indicator
**What:** If all voltage readings drop to 0 simultaneously across all devices, surface a banner: "Grid appears to be down since [timestamp]." Clear automatically when voltage returns.
**Why:** Useful real-time information, especially post-storm. Derived entirely from existing data.
**Depends on:** SSE hub (for real-time banner update without page refresh)

---

### [P3/S] Carbon offset counter
**What:** Display kg CO2 avoided today/this month/lifetime based on kWh produced × national grid emission factor (static constant per country).
**Why:** Feel-good metric that increases user engagement. Data already exists.
**Depends on:** Core monitoring pipeline
