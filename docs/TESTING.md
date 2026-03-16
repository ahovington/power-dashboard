# Testing Strategy

## Overview

This project follows Test-Driven Development (TDD) principles with comprehensive unit, integration, and end-to-end testing.

## Backend Testing (Go)

### Testing Tools

- **Testing Framework**: Go's built-in `testing` package
- **Assertions**: `github.com/stretchr/testify/assert`
- **Mocking**: `github.com/golang/mock` (GoMock)
- **HTTP mocking**: `net/http/httptest` (never call real provider URLs in tests)
- **Test Fixtures**: Custom mock responses

### Unit Testing

#### Service Layer Tests

```go
// internal/service/power_service_test.go
package service_test

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestGetPowerMetrics(t *testing.T) {
    // Arrange
    mockAdapter := NewMockProviderAdapter()
    service := NewPowerService(mockAdapter)

    // Act
    metrics, err := service.GetPowerMetrics(context.Background())

    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, metrics)
    assert.Greater(t, metrics.TotalGenerated, 0)
}
```

#### Repository Tests

```go
// internal/repository/reading_repository_test.go
func TestSaveReading(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    repo := NewReadingRepository(db)
    reading := &PowerReading{...}

    err := repo.Save(context.Background(), reading)
    assert.NoError(t, err)
}

func TestSaveReading_DuplicateIsIgnored(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    repo := NewReadingRepository(db)
    reading := &PowerReading{DeviceID: uuid.New(), ReadingTimestamp: time.Now()}

    err := repo.Save(context.Background(), reading)
    assert.NoError(t, err)

    // Second save of same (device_id, reading_timestamp) must not error
    err = repo.Save(context.Background(), reading)
    assert.NoError(t, err, "ON CONFLICT DO NOTHING should silently skip duplicate")
}
```

#### Ingestion Service Tests

```go
// internal/service/ingestion_service_test.go

func TestIngestionService_RateLimitTriggersBackoff(t *testing.T) {
    mockAdapter := NewMockProviderAdapter()
    mockAdapter.EXPECT().GetSystemStatus(gomock.Any()).Return(nil, adapter.ErrRateLimited)

    svc := NewIngestionService(mockAdapter, mockRepo, eventBus, 100*time.Millisecond)

    ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
    defer cancel()
    svc.RunPoller(ctx)

    // Assert it did not call the adapter more than once in the window
    // (backoff kicked in)
}

func TestIngestionService_PanicIsRecovered(t *testing.T) {
    mockAdapter := NewMockProviderAdapter()
    mockAdapter.EXPECT().GetSystemStatus(gomock.Any()).DoAndReturn(func(_ context.Context) (*adapter.SystemStatus, error) {
        panic("simulated nil pointer dereference")
    })

    svc := NewIngestionService(mockAdapter, mockRepo, eventBus, 50*time.Millisecond)

    // Should not propagate the panic
    assert.NotPanics(t, func() {
        ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
        defer cancel()
        svc.RunPoller(ctx)
    })
}

func TestIngestionService_GracefulShutdown(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    svc := NewIngestionService(mockAdapter, mockRepo, eventBus, 1*time.Second)

    done := make(chan struct{})
    go func() {
        svc.RunPoller(ctx)
        close(done)
    }()

    cancel()

    select {
    case <-done:
        // ok
    case <-time.After(2 * time.Second):
        t.Fatal("RunPoller did not stop within 2s of context cancellation")
    }
}
```

### Adapter Tests (with httptest.Server)

All adapter tests use `httptest.NewServer` to mock provider responses. **Never call real provider URLs in tests.**

```go
// pkg/enphase/adapter_test.go
package enphase_test

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "yourmodule/pkg/adapter"
    "yourmodule/pkg/enphase"
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
    assert.Greater(t, status.PowerProduced, 0)
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

func TestGetSystemStatus_MalformedJSON(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("not valid json"))
    }))
    defer srv.Close()

    a := enphase.NewAdapter(enphase.Config{APIKey: "test", BaseURL: srv.URL})

    _, err := a.GetSystemStatus(context.Background())

    assert.Error(t, err)
}
```

### SSE Hub Tests

```go
// internal/api/sse_test.go
package api_test

import (
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
)

func TestHub_SingleClientReceivesEvents(t *testing.T) {
    hub := NewHub()
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go hub.Run(ctx)

    ch := hub.Subscribe()
    defer hub.Unsubscribe(ch)

    event := PowerEvent{PowerProduced: 5000}
    hub.Broadcast(event)

    select {
    case received := <-ch:
        assert.Equal(t, 5000, received.PowerProduced)
    case <-time.After(time.Second):
        t.Fatal("did not receive event within 1s")
    }
}

func TestHub_SlowClientDoesNotBlockOthers(t *testing.T) {
    hub := NewHub()
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go hub.Run(ctx)

    // Fast client
    fast := hub.Subscribe()
    defer hub.Unsubscribe(fast)

    // Slow client: never reads from the channel — buffer fills up
    slow := hub.Subscribe()
    defer hub.Unsubscribe(slow)
    _ = slow // intentionally not reading

    // Broadcast more events than the slow client's buffer
    for i := 0; i < 20; i++ {
        hub.Broadcast(PowerEvent{PowerProduced: i * 100})
    }

    // Fast client should still receive events
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
    assert.Greater(t, received, 0, "fast client should have received events")
}
```

### Mock API Responses

```go
// pkg/enphase/mock_responses.go
package enphase

func MockSystemStatusResponse() *SystemStatusResponse {
    return &SystemStatusResponse{
        System: SystemInfo{
            ID:     "test-system-123",
            Name:   "Test System",
            Status: "normal",
        },
        Modules:       20,
        Production:    5000,
        Consumption:   3000,
        NetProduction: 2000,
    }
}

func MockPowerMetricsResponse() *PowerMetricsResponse {
    return &PowerMetricsResponse{
        Interval: 300,
        Readings: []Reading{
            {
                Timestamp:   time.Now(),
                Production:  5000,
                Consumption: 3000,
            },
        },
    }
}
```

### Integration Testing

```go
// tests/integration/power_service_integration_test.go
func TestPowerServiceWithDatabase(t *testing.T) {
    db := setupTestDatabase(t)
    defer teardownTestDatabase(db)

    repo := repository.NewReadingRepository(db)
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(enphase.MockPowerMetricsResponse())
    }))
    defer srv.Close()

    mockAdapter := enphase.NewAdapter(enphase.Config{APIKey: "test", BaseURL: srv.URL})
    svc := service.NewPowerService(repo, mockAdapter)

    err := svc.SyncPowerMetrics(context.Background())
    assert.NoError(t, err)

    // Verify data was persisted
    readings, err := repo.GetLatestReadings(context.Background(), 10)
    assert.NoError(t, err)
    assert.NotEmpty(t, readings, "expected at least one reading after sync")
}
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run with race detector (required for CI — catches goroutine races in SSE hub, ingestion)
go test -race ./...

# Run with coverage
go test -cover ./...

# Generate HTML coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Run specific test
go test -run TestGetPowerMetrics ./...

# Run with verbose output
go test -v ./...

# Run with timeout
go test -timeout 30s ./...
```

## Frontend Testing (React)

### Testing Tools

- **Test Runner**: Jest
- **Testing Library**: React Testing Library
- **Mocking**: MSW (Mock Service Worker)
- **Snapshots**: Jest snapshots for component structure

### Unit Testing Components

```typescript
// src/components/__tests__/Dashboard.test.tsx
import { render, screen } from '@testing-library/react';
import Dashboard from '../Dashboard';

describe('Dashboard', () => {
  it('renders power metrics', () => {
    render(<Dashboard />);

    expect(screen.getByText(/power consumption/i)).toBeInTheDocument();
  });

  it('displays real-time updates', async () => {
    render(<Dashboard />);

    const powerValue = screen.getByTestId('current-power');
    expect(powerValue).toHaveTextContent('5.2 kW');
  });
});
```

### Testing Hooks

```typescript
// src/hooks/__tests__/usePowerData.test.ts
import { renderHook, waitFor } from '@testing-library/react';
import usePowerData from '../usePowerData';

describe('usePowerData', () => {
  it('fetches power data on mount', async () => {
    const { result } = renderHook(() => usePowerData());

    await waitFor(() => {
      expect(result.current.data).toBeDefined();
      expect(result.current.loading).toBe(false);
    });
  });
});
```

### Mock Service Worker (MSW)

```typescript
// src/tests/handlers.ts
import { http, HttpResponse } from 'msw';

export const handlers = [
  http.get('/api/v1/power/readings', () => {
    return HttpResponse.json({
      powerGenerated: 5200,
      powerConsumed: 3100,
      powerNet: 2100,
      timestamp: new Date().toISOString(),
    });
  }),

  http.get('/api/v1/battery/status', () => {
    return HttpResponse.json({
      chargePercent: 85,
      powerFlowing: 500,
      direction: 'charging',
      temperature: 25,
    });
  }),
];
```

### Running Frontend Tests

```bash
# Run all tests
npm test

# Run in watch mode
npm test -- --watch

# Run with coverage
npm test -- --coverage

# Run specific test file
npm test Dashboard.test.tsx

# Update snapshots
npm test -- -u
```

## Test Coverage Goals

| Category | Target |
|----------|--------|
| Backend Services | 80%+ |
| Backend Handlers | 70%+ |
| Backend Adapters | 90%+ (typed error paths must all be covered) |
| Frontend Components | 75%+ |
| Frontend Hooks | 80%+ |
| Overall | 75%+ |

Check coverage:

```bash
# Backend
go test -cover ./... | grep total

# Frontend
npm test -- --coverage
```

## Mock Data Fixtures

### Power Reading Fixtures

```go
// tests/fixtures/power_readings.go
package fixtures

func SamplePowerReading() *model.PowerReading {
    return &model.PowerReading{
        ID:                  uuid.New(),
        DeviceID:            uuid.New(),
        ReadingTimestamp:    time.Now().UTC(), // always UTC
        PowerProduced:       5000,
        PowerConsumed:       3000,
        // power_net is computed on read — not stored
        EnergyProducedToday: 45000,
        EnergyConsumedToday: 28000,
    }
}

func SampleBatteryStatus() *model.BatteryStatus {
    return &model.BatteryStatus{
        ID:               uuid.New(),
        DeviceID:         uuid.New(),
        ReadingTimestamp: time.Now().UTC(),
        ChargePercentage: 85.5,
        PowerFlowing:     500,
        PowerDirection:   "charging",
        Temperature:      25.3,
    }
}
```

## Continuous Integration

### GitHub Actions

```yaml
# .github/workflows/test.yml
name: Tests

on: [push, pull_request]

jobs:
  backend:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:14
        env:
          POSTGRES_PASSWORD: password
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: 1.21
      - run: go test ./...
      - run: go test -race ./...    # Required: catches goroutine races
      - run: go test -cover ./...

  frontend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
        with:
          node-version: 18
      - run: npm ci
      - run: npm test -- --coverage --watchAll=false
```

## Best Practices

1. **Test Naming**: Clear, descriptive test names that explain what is being tested
2. **Arrange-Act-Assert**: Follow AAA pattern in tests
3. **Test Isolation**: Each test should be independent
4. **Mock External Dependencies**: Always use `httptest.NewServer` for HTTP — never call real provider URLs
5. **Avoid Implementation Details**: Test behavior, not implementation
6. **Keep Tests Fast**: Mock slow operations
7. **DRY Principle**: Extract common test setup to helper functions
8. **Error Cases**: Test all three typed error paths per adapter method (`ErrRateLimited`, `ErrAuthExpired`, `ErrProviderUnavailable`) plus malformed JSON
9. **Race detector**: Always run `go test -race` in CI for concurrent code (ingestion, SSE hub)
10. **UTC timestamps**: All fixture timestamps use `.UTC()` — never `time.Now()` without timezone

## Debugging Tests

### Backend

```bash
# Run tests with verbose output
go test -v -run TestName ./...

# Run with debug logging
GO_LOG=debug go test ./...

# Use debugger
dlv test ./package -- -test.v -test.run TestName
```

### Frontend

```bash
# Debug single test
npm test -- --testNamePattern="test name"

# Run with Node debugger
node --inspect-brk node_modules/.bin/jest --runInBand
```

## Resources

- [Go Testing Best Practices](https://golang.org/doc/effective_go#testing)
- [httptest Package](https://pkg.go.dev/net/http/httptest)
- [React Testing Library Docs](https://testing-library.com/react)
- [Jest Documentation](https://jestjs.io/)
- [MSW Documentation](https://mswjs.io/)
