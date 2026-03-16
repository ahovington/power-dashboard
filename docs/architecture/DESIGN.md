# Architecture Design

## Overview

The Household Power Monitor uses a layered, modular architecture designed for scalability and extensibility across multiple energy API providers.

## System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Frontend (React)                         │
│  Dashboard | Metrics | Battery Status | Alerts              │
│                                                              │
│  REST polling (/api/v1/...)    SSE stream (/api/v1/events)  │
└─────────────────────────────────────────────────────────────┘
                              ↕
┌─────────────────────────────────────────────────────────────┐
│                  Backend API (Go)                            │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ HTTP Layer (Routes /api/v1/..., Handlers)            │   │
│  │ SSE Hub (fan-out to connected browser clients)       │   │
│  └──────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Service Layer (Business Logic)                       │   │
│  │ ├── PowerService (query, aggregate)                  │   │
│  │ └── IngestionService (background goroutine)          │   │
│  └──────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Adapter Layer (Provider Abstraction)                 │   │
│  │ ├── Enphase Adapter                                  │   │
│  │ ├── Tesla Adapter (Future)                           │   │
│  │ └── Generic HTTP Client                              │   │
│  └──────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Repository Layer (Data Access)                       │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              ↕
┌─────────────────────────────────────────────────────────────┐
│              PostgreSQL Database                             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Power Readings | Devices | Metrics | Alerts          │   │
│  │ Users | Households                                   │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Data Flow

### Ingestion (background)

```
External Provider API (Enphase, Tesla...)
            │
            │  HTTP poll (configurable interval, default 5 min)
            ▼
  IngestionService.RunPoller(ctx)
  ┌─────────────────────────────────────┐
  │  recover() wraps each cycle         │ ← panic-safe
  │  exponential backoff on error       │ ← 1s→2s→4s...5min cap
  │  graceful shutdown via ctx.Done()   │
  └─────────────────────────────────────┘
            │
     ┌──────┴──────┐
     ▼             ▼
PostgreSQL      EventBus (chan PowerEvent)
(persist)            │
                     │  fan-out
                     ▼
               SSE Hub → Client A
                       → Client B
                       → Client N
```

### Query (request/response)

```
Frontend GET /api/v1/power/readings?device_id=...&start=...&end=...
    │
    ▼
Handler → PowerService.GetReadings() → Repository.GetReadings()
    │                                        │
    │                                        ▼
    │                                   PostgreSQL
    │                                   (DATE_TRUNC aggregation
    │                                    for chart queries)
    ▼
JSON response
```

### Real-time updates (SSE)

```
Frontend GET /api/v1/events
    │  (long-lived HTTP connection)
    ▼
SSE Handler → Hub.Subscribe(clientChan)
                    │
                    │  receives PowerEvent from ingestion
                    ▼
             write event to client
             (buffered channel — slow client doesn't block hub)
```

## Design Patterns

### 1. Adapter Pattern

Abstracts different API providers behind a common interface. Each provider implements `ProviderAdapter` with typed configuration — no `interface{}`.

```
ProviderAdapter interface
     ↑
     ├── EnphaseAdapter  (config: EnphaseConfig{APIKey, SystemID})
     ├── TeslaAdapter    (config: TeslaConfig{Token, SiteID})
     └── NewProvider     (config: NewProviderConfig{...})
```

### 2. Strategy Pattern

Different data retrieval strategies based on provider capabilities (polling vs webhooks).

### 3. Repository Pattern

Abstraction for data persistence. All time-series inserts use `ON CONFLICT DO NOTHING` to handle re-polling.

### 4. Service Layer Pattern

Business logic separated from HTTP and data access. The `IngestionService` owns the background goroutine lifecycle.

## API Versioning

All API routes are prefixed with `/api/v1/`. This is established from day one to avoid breaking frontend clients when the API evolves.

```
GET  /api/v1/power/readings
GET  /api/v1/power/status
GET  /api/v1/battery/status
GET  /api/v1/devices
GET  /api/v1/alerts
GET  /api/v1/events          ← SSE endpoint (text/event-stream)
GET  /metrics                ← Prometheus metrics (not versioned)
```

## Backend Package Structure

```
backend/
├── cmd/server/
│   └── main.go              # Entry point, starts HTTP server + ingestion goroutine
├── internal/
│   ├── api/
│   │   ├── routes.go        # HTTP routes (all under /api/v1/)
│   │   ├── handler.go       # Request handlers
│   │   └── sse.go           # SSE hub + /api/v1/events handler
│   ├── service/
│   │   ├── power_service.go    # Query + aggregate business logic
│   │   ├── ingestion_service.go # Background poll goroutine
│   │   └── interfaces.go       # Service interfaces
│   ├── repository/
│   │   ├── reading_repo.go  # power_readings (ON CONFLICT DO NOTHING)
│   │   └── device_repo.go
│   ├── model/
│   │   ├── power_reading.go # Domain models
│   │   └── device.go
│   └── config/
│       └── config.go        # Loads provider keys from env vars
├── pkg/
│   ├── enphase/            # Enphase provider implementation
│   ├── adapter/            # ProviderAdapter interface + typed errors
│   └── common/             # Shared utilities
└── tests/
    ├── unit/
    ├── integration/
    └── fixtures/           # Mock data
```

## Key Interfaces

### ProviderAdapter Interface

```go
// pkg/adapter/provider_adapter.go

// Each provider has its own typed config struct — no interface{} parameters.
type ProviderAdapter interface {
    GetSystemStatus(ctx context.Context) (*SystemStatus, error)
    GetPowerMetrics(ctx context.Context, duration time.Duration) ([]PowerMetrics, error)
    GetDeviceList(ctx context.Context) ([]DeviceInfo, error)
    GetBatteryStatus(ctx context.Context) (*BatteryStatus, error)
    GetPowerQuality(ctx context.Context) (*PowerQualityMetrics, error)
}

// Authenticate is separate from data retrieval.
// Each adapter's constructor accepts its typed config and handles auth internally.
// Example:
//   enphase.NewAdapter(EnphaseConfig{APIKey: "...", SystemID: "..."})
```

### Typed Errors

```go
// pkg/adapter/errors.go
var (
    ErrRateLimited       = errors.New("provider: rate limit exceeded")
    ErrAuthExpired       = errors.New("provider: authentication expired")
    ErrProviderUnavailable = errors.New("provider: service unavailable")
)
```

### IngestionService

```go
// internal/service/ingestion_service.go

type IngestionService struct {
    adapter    adapter.ProviderAdapter
    repo       ReadingRepository
    eventBus   chan<- PowerEvent
    interval   time.Duration
}

// RunPoller blocks until ctx is cancelled (call in a goroutine from main).
// Panics are recovered and logged; errors trigger exponential backoff.
func (s *IngestionService) RunPoller(ctx context.Context)
```

### SSE Hub

```go
// internal/api/sse.go

type Hub struct {
    subscribe   chan subscription
    unsubscribe chan subscription
    broadcast   chan PowerEvent
}

// Subscribe returns a buffered channel that receives PowerEvents.
// The buffer prevents a slow client from blocking the hub's broadcast loop.
func (h *Hub) Subscribe() <-chan PowerEvent
func (h *Hub) Unsubscribe(ch <-chan PowerEvent)
func (h *Hub) Run(ctx context.Context) // goroutine
```

## Dependencies

- **Web Framework**: Chi router
- **Database**: pgx (PostgreSQL driver)
- **Migrations**: golang-migrate/migrate
- **Testing**: testify, GoMock, httptest
- **Logging**: slog (stdlib) with structured fields
- **Metrics**: promhttp (Prometheus)
- **JSON**: encoding/json (stdlib)
- **HTTP Client**: net/http (stdlib)

## Frontend Architecture

- **State Management**: React Context + Hooks (scoped; avoid global Context for high-frequency updates)
- **Data Fetching**: React Query for REST endpoints (device list, historical data, alerts)
- **Real-time**: Native `EventSource` API for SSE live power metrics
- **Styling**: Tailwind CSS
- **Charts**: Recharts for visualization
- **Component Library**: Headless UI

## Security Considerations

1. **API Keys**: Stored in environment variables only — never in code or the database
2. **Authentication**: Token-based auth for backend APIs
3. **HTTPS**: Required for production
4. **CORS**: Explicitly configured for the frontend origin only (not `*`)
5. **Input Validation**: All inputs validated server-side before reaching the service layer
6. **Rate Limiting**: Implement rate limiting on API endpoints
7. **Typed adapters**: No `interface{}` in adapter interfaces — type safety enforced at compile time

## Performance Considerations

1. **Database Indexing**: Composite indexes on `(device_id, reading_timestamp DESC)` for all time-series tables
2. **Aggregation at the DB**: Use `DATE_TRUNC` + `AVG/SUM` for chart queries — never return raw rows for time-range requests
3. **SSE backpressure**: Buffered channels per client; slow clients are dropped rather than stalling the broadcast loop
4. **Connection Pooling**: pgx connection pool (min 5, max 20)
5. **Provider Rate Limits**: Ingestion goroutine respects provider limits; uses exponential backoff on 429 responses

## Observability

Structured log fields on every ingestion cycle:

```
level=INFO msg="ingestion.cycle.complete" provider=enphase device_id=... readings_persisted=12 duration_ms=340 next_poll_in=5m
level=ERROR msg="ingestion.cycle.failed" provider=enphase error="rate limit exceeded" retry_in=30s
```

Prometheus metrics at `GET /metrics`:
- `ingestion_cycle_duration_ms` — poll latency
- `provider_api_errors_total{provider,status_code}` — provider health
- `power_readings_persisted_total` — data flow confirmation
- `sse_connected_clients` — dashboard load

## Scalability

Future enhancements (see TODOS.md):
- Horizontal scaling with load balancer (SSE hub must move to Redis pub/sub for multi-instance)
- TimescaleDB for automatic data retention and continuous aggregates
- Message queue for async processing
- GraphQL API with subscriptions
