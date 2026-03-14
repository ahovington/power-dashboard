# Architecture Design

## Overview

The Household Power Monitor uses a layered, modular architecture designed for scalability and extensibility across multiple energy API providers.

## System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Frontend (React)                         │
│  Dashboard | Metrics | Battery Status | Alerts              │
└─────────────────────────────────────────────────────────────┘
                              ↕
┌─────────────────────────────────────────────────────────────┐
│                  Backend API (Go)                            │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ HTTP Layer (Routes, Handlers)                        │   │
│  └──────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Service Layer (Business Logic)                       │   │
│  └──────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Adapter Layer (Provider Abstraction)                 │   │
│  │ ├── Enphase Adapter                                  │   │
│  │ ├── Tesla Adapter (Future)                           │   │
│  │ └── Generic API Client                               │   │
│  └──────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Repository Layer (Data Access)                       │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              ↕
┌─────────────────────────────────────────────────────────────┐
│              PostgreSQL Database                             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Power Readings | Devices | Metrics | Alerts         │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Design Patterns

### 1. **Adapter Pattern**
Used to abstract different API providers (Enphase, Tesla, etc.) behind a common interface.

```
Provider Interface
     ↑
     ├── Enphase Adapter
     ├── Tesla Adapter
     └── Generic Adapter
```

### 2. **Strategy Pattern**
Different data retrieval strategies based on provider capabilities.

### 3. **Repository Pattern**
Abstraction for data persistence, allowing easy testing and switching storage backends.

### 4. **Service Layer Pattern**
Business logic separated from HTTP and data access layers.

## Data Flow

1. **API Request** → Frontend makes request to Backend API
2. **Service Layer** → Service determines which provider adapter to use
3. **Provider Adapter** → Adapter translates request to provider-specific API call
4. **External API** → Call to Enphase/Tesla API
5. **Data Transformation** → Response transformed to common format
6. **Repository** → Data persisted to PostgreSQL
7. **Response** → Data returned to frontend

## Extensibility Strategy

### Adding a New Provider

1. Create new adapter in `pkg/[provider]/`
2. Implement `ProviderAdapter` interface
3. Register adapter in service layer
4. Add provider-specific configuration
5. Create tests with mock responses

### Example: Adding Tesla Integration

```
pkg/tesla/
├── client.go          # Tesla API client
├── adapter.go         # Adapter implementation
├── types.go          # Tesla-specific types
└── mock_responses.go # Test data
```

## Backend Package Structure

```
backend/
├── cmd/server/
│   └── main.go              # Entry point
├── internal/
│   ├── api/
│   │   ├── routes.go        # HTTP routes
│   │   └── handler.go       # Request handlers
│   ├── service/
│   │   ├── power_service.go # Core business logic
│   │   └── interfaces.go    # Service interfaces
│   ├── repository/
│   │   ├── reading_repo.go  # Data access
│   │   └── device_repo.go
│   ├── model/
│   │   ├── power_reading.go # Domain models
│   │   └── device.go
│   └── config/
│       └── config.go        # Configuration
├── pkg/
│   ├── enphase/            # Enphase provider
│   ├── adapter/            # Adapter interface
│   └── common/             # Shared utilities
└── tests/
    ├── unit/
    ├── integration/
    └── fixtures/           # Mock data
```

## Key Interfaces

### ProviderAdapter Interface

```go
type ProviderAdapter interface {
    GetSystemStatus(ctx context.Context) (*SystemStatus, error)
    GetPowerMetrics(ctx context.Context, duration time.Duration) (*PowerMetrics, error)
    GetDeviceList(ctx context.Context) ([]*Device, error)
    Authenticate(ctx context.Context, credentials interface{}) error
}
```

## Dependencies

- **Web Framework**: Chi router
- **Database**: pgx (PostgreSQL driver)
- **Testing**: testify, GoMock
- **Logging**: slog (stdlib)
- **JSON**: encoding/json (stdlib)
- **HTTP Client**: net/http (stdlib)

## Frontend Architecture

- **State Management**: React Context + Hooks
- **Data Fetching**: React Query for caching and synchronization
- **Styling**: Tailwind CSS
- **Charts**: Recharts for visualization
- **Component Library**: Headless UI

## Security Considerations

1. **API Keys**: Stored in environment variables, never in code
2. **Authentication**: Token-based auth for backend APIs
3. **HTTPS**: Required for production
4. **CORS**: Properly configured for frontend domain
5. **Input Validation**: All inputs validated server-side
6. **Rate Limiting**: Implement rate limiting on API endpoints

## Performance Considerations

1. **Database Indexing**: Indexes on timestamp and device_id
2. **Caching**: In-memory cache for frequently accessed data
3. **Connection Pooling**: Database connection pooling
4. **Frontend Optimization**: Code splitting and lazy loading
5. **API Rate Limiting**: Respect provider API limits

## Scalability

Future enhancements:
- Horizontal scaling with load balancer
- Redis caching layer
- Message queue for async processing
- Time-series database (InfluxDB) for metrics
- GraphQL API option