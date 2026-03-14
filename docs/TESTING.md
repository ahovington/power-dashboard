# Testing Strategy

## Overview

This project follows Test-Driven Development (TDD) principles with comprehensive unit, integration, and end-to-end testing.

## Backend Testing (Go)

### Testing Tools

- **Testing Framework**: Go's built-in `testing` package
- **Assertions**: `github.com/stretchr/testify/assert`
- **Mocking**: `github.com/golang/mock` (GoMock)
- **Test Fixtures**: Custom mock responses

### Unit Testing

#### Service Layer Tests

```go
// internal/service/power_service_test.go
package service

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
```

### Mock API Responses

#### Enphase Mock Responses

```go
// pkg/enphase/mock_responses.go
package enphase

func MockSystemStatusResponse() *SystemStatusResponse {
    return &SystemStatusResponse{
        System: SystemInfo{
            ID: "test-system-123",
            Name: "Test System",
            Status: "normal",
        },
        Modules: 20,
        Production: 5000,
        Consumption: 3000,
        NetProduction: 2000,
    }
}

func MockPowerMetricsResponse() *PowerMetricsResponse {
    return &PowerMetricsResponse{
        Interval: 300,
        Readings: []Reading{
            {
                Timestamp: time.Now(),
                Production: 5000,
                Consumption: 3000,
                StoredEnergy: 8500,
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
    mockAdapter := NewMockEnphaseAdapter()
    service := service.NewPowerService(repo, mockAdapter)
    
    // Test end-to-end flow
    err := service.SyncPowerMetrics(context.Background())
    assert.NoError(t, err)
    
    // Verify data was persisted
    readings, _ := repo.GetLatestReadings(context.Background(), 10)
    assert.Equal(t, 10, len(readings))
}
```

### Running Tests

```bash
# Run all tests
go test ./...

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
  http.get('/api/power/metrics', () => {
    return HttpResponse.json({
      powerGenerated: 5200,
      powerConsumed: 3100,
      powerNet: 2100,
      timestamp: new Date().toISOString(),
    });
  }),
  
  http.get('/api/battery/status', () => {
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
        ID: uuid.New(),
        DeviceID: uuid.New(),
        ReadingTimestamp: time.Now(),
        PowerProduced: 5000,
        PowerConsumed: 3000,
        PowerNet: 2000,
        EnergyProducedToday: 45000,
        EnergyConsumedToday: 28000,
    }
}

func SampleBatteryStatus() *model.BatteryStatus {
    return &model.BatteryStatus{
        ID: uuid.New(),
        DeviceID: uuid.New(),
        ChargePercentage: 85.5,
        PowerFlowing: 500,
        PowerDirection: "charging",
        Temperature: 25.3,
    }
}
```

## Continuous Integration

### GitHub Actions Example

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
      - run: go test -cover ./...
      - run: go test -race ./...

  frontend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
        with:
          node-version: 18
      - run: npm ci
      - run: npm test -- --coverage
```

## Best Practices

1. **Test Naming**: Clear, descriptive test names that explain what is being tested
2. **Arrange-Act-Assert**: Follow AAA pattern in tests
3. **Test Isolation**: Each test should be independent
4. **Mock External Dependencies**: Always mock API calls and database
5. **Avoid Implementation Details**: Test behavior, not implementation
6. **Keep Tests Fast**: Mock slow operations
7. **DRY Principle**: Extract common test setup to helper functions
8. **Error Cases**: Test both happy path and error scenarios

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
- [React Testing Library Docs](https://testing-library.com/react)
- [Jest Documentation](https://jestjs.io/)
- [MSW Documentation](https://mswjs.io/)