# API Integration Guide

## Overview

This guide explains how to integrate new energy API providers into the Power Monitor system.

## Architecture

The system uses the **Adapter Pattern** to abstract different provider implementations. Each adapter is constructed with a typed config struct — `interface{}` is never used in adapter signatures.

```
ProviderAdapter interface
     ↑
     ├── EnphaseAdapter  (constructed with EnphaseConfig)
     ├── TeslaAdapter    (constructed with TeslaConfig)
     └── NewProviderAdapter (constructed with NewProviderConfig)
```

## Creating a New Provider Adapter

### Step 1: Define the Adapter Interface

All providers implement the `ProviderAdapter` interface from `pkg/adapter/provider_adapter.go`:

```go
// pkg/adapter/provider_adapter.go
package adapter

import (
    "context"
    "time"
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
    // power_net is computed by callers as PowerProduced - PowerConsumed
    Frequency     float64
    VoltagePhaseA float64
    VoltagePhaseB float64
    VoltagePhaseC float64
}

type ProviderAdapter interface {
    // Get current system status
    GetSystemStatus(ctx context.Context) (*SystemStatus, error)

    // Get power metrics for time period
    GetPowerMetrics(ctx context.Context, duration time.Duration) ([]PowerMetrics, error)

    // Get list of connected devices
    GetDeviceList(ctx context.Context) ([]DeviceInfo, error)

    // Get battery status (if available)
    GetBatteryStatus(ctx context.Context) (*BatteryStatus, error)

    // Get power quality metrics
    GetPowerQuality(ctx context.Context) (*PowerQualityMetrics, error)
}
```

Authentication is handled in the adapter constructor, not as an interface method. Each adapter manages its own token lifecycle internally.

### Typed Error Sentinels

```go
// pkg/adapter/errors.go
package adapter

import "errors"

var (
    ErrRateLimited         = errors.New("provider: rate limit exceeded")
    ErrAuthExpired         = errors.New("provider: authentication expired")
    ErrProviderUnavailable = errors.New("provider: service unavailable")
)
```

### Step 2: Create Provider Package

```bash
mkdir -p backend/pkg/newprovider
```

### Step 3: Define Typed Config

```go
// pkg/newprovider/config.go
package newprovider

// NewProviderConfig holds credentials loaded from environment variables.
// Never accept interface{} — typed config is enforced at compile time.
type NewProviderConfig struct {
    APIKey  string
    BaseURL string // optional override for testing
}
```

### Step 4: Implement the Adapter

```go
// pkg/newprovider/adapter.go
package newprovider

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "yourmodule/pkg/adapter"
)

type NewProviderAdapter struct {
    config     NewProviderConfig
    httpClient *http.Client
}

func NewAdapter(cfg NewProviderConfig) *NewProviderAdapter {
    if cfg.BaseURL == "" {
        cfg.BaseURL = "https://api.newprovider.com/v1"
    }
    return &NewProviderAdapter{
        config: cfg,
        httpClient: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}

func (a *NewProviderAdapter) GetSystemStatus(ctx context.Context) (*adapter.SystemStatus, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", a.config.BaseURL+"/system/status", nil)
    if err != nil {
        return nil, fmt.Errorf("newprovider: build request: %w", err)
    }

    req.Header.Set("Authorization", "Bearer "+a.config.APIKey)

    resp, err := a.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("newprovider: do request: %w", err)
    }
    defer resp.Body.Close()

    // Always check the status code before decoding.
    switch resp.StatusCode {
    case http.StatusOK:
        // continue
    case http.StatusUnauthorized, http.StatusForbidden:
        return nil, adapter.ErrAuthExpired
    case http.StatusTooManyRequests:
        return nil, adapter.ErrRateLimited
    default:
        return nil, fmt.Errorf("%w: HTTP %d", adapter.ErrProviderUnavailable, resp.StatusCode)
    }

    var statusResp SystemStatusResponse
    if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
        return nil, fmt.Errorf("newprovider: decode response: %w", err)
    }

    return &adapter.SystemStatus{
        ID:            statusResp.SystemID,
        Name:          statusResp.Name,
        Status:        statusResp.State,
        PowerProduced: statusResp.CurrentProduction,
        PowerConsumed: statusResp.CurrentConsumption,
    }, nil
}

// Implement other interface methods...
```

### Step 5: Create Type Definitions

```go
// pkg/newprovider/types.go
package newprovider

type SystemStatusResponse struct {
    SystemID           string `json:"system_id"`
    Name               string `json:"name"`
    State              string `json:"state"`
    CurrentProduction  int    `json:"current_production"`
    CurrentConsumption int    `json:"current_consumption"`
}

type MetricsResponse struct {
    Timestamp   string  `json:"timestamp"`
    Production  int     `json:"production"`
    Consumption int     `json:"consumption"`
    Frequency   float64 `json:"frequency"`
}
```

### Step 6: Create Mock Responses

```go
// pkg/newprovider/mock_responses.go
package newprovider

func MockSystemStatus() *SystemStatusResponse {
    return &SystemStatusResponse{
        SystemID:           "mock-system-123",
        Name:               "Test System",
        State:              "normal",
        CurrentProduction:  5000,
        CurrentConsumption: 3000,
    }
}
```

### Step 7: Register Adapter in Service Factory

```go
// internal/service/provider_factory.go
package service

import (
    "fmt"
    "yourmodule/pkg/adapter"
    "yourmodule/pkg/enphase"
    "yourmodule/pkg/newprovider"
)

// CreateAdapter uses typed config structs — no interface{} type assertions.
func CreateAdapter(providerType string, env map[string]string) (adapter.ProviderAdapter, error) {
    switch providerType {
    case "enphase":
        return enphase.NewAdapter(enphase.Config{
            APIKey:   env["ENPHASE_API_KEY"],
            SystemID: env["ENPHASE_SYSTEM_ID"],
        }), nil
    case "newprovider":
        return newprovider.NewAdapter(newprovider.NewProviderConfig{
            APIKey: env["NEWPROVIDER_API_KEY"],
        }), nil
    default:
        return nil, fmt.Errorf("unsupported provider: %q", providerType)
    }
}
```

### Step 8: Add Tests

Tests use `httptest.NewServer` to mock provider responses — never call real provider URLs in tests.

```go
// pkg/newprovider/adapter_test.go
package newprovider_test

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "yourmodule/pkg/adapter"
    "yourmodule/pkg/newprovider"
)

func TestGetSystemStatus_OK(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(newprovider.MockSystemStatus())
    }))
    defer srv.Close()

    a := newprovider.NewAdapter(newprovider.NewProviderConfig{
        APIKey:  "test-key",
        BaseURL: srv.URL,
    })

    status, err := a.GetSystemStatus(context.Background())

    require.NoError(t, err)
    assert.Equal(t, "mock-system-123", status.ID)
    assert.Equal(t, 5000, status.PowerProduced)
}

func TestGetSystemStatus_RateLimit(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusTooManyRequests)
    }))
    defer srv.Close()

    a := newprovider.NewAdapter(newprovider.NewProviderConfig{APIKey: "test", BaseURL: srv.URL})

    _, err := a.GetSystemStatus(context.Background())

    assert.ErrorIs(t, err, adapter.ErrRateLimited)
}

func TestGetSystemStatus_AuthExpired(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusUnauthorized)
    }))
    defer srv.Close()

    a := newprovider.NewAdapter(newprovider.NewProviderConfig{APIKey: "expired", BaseURL: srv.URL})

    _, err := a.GetSystemStatus(context.Background())

    assert.ErrorIs(t, err, adapter.ErrAuthExpired)
}

func TestGetSystemStatus_MalformedJSON(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("not json"))
    }))
    defer srv.Close()

    a := newprovider.NewAdapter(newprovider.NewProviderConfig{APIKey: "test", BaseURL: srv.URL})

    _, err := a.GetSystemStatus(context.Background())

    assert.Error(t, err)
}
```

## Integration Checklist

- [ ] Typed config struct defined (no `interface{}` parameters)
- [ ] Constructor accepts typed config, handles auth internally
- [ ] All HTTP response status codes checked before decoding (200, 401, 429, 5xx)
- [ ] Typed sentinel errors returned (`ErrRateLimited`, `ErrAuthExpired`, `ErrProviderUnavailable`)
- [ ] Errors wrapped with `fmt.Errorf("provider: action: %w", err)` for context
- [ ] `httptest.NewServer` used in all tests — no real HTTP calls
- [ ] Tests cover: 200 valid, 200 invalid JSON, 401, 429, 5xx, network timeout
- [ ] Mock responses defined in `mock_responses.go`
- [ ] Adapter registered in `provider_factory.go` with typed config
- [ ] Integration tests with service layer
- [ ] API credentials documented in `.env.example`
- [ ] Logging: provider name included in all error log lines

## Common Provider API Patterns

### REST API

```go
type RESTProvider struct {
    config NewProviderConfig
    client *http.Client
}

func (p *RESTProvider) get(ctx context.Context, endpoint string, out interface{}) error {
    req, err := http.NewRequestWithContext(ctx, "GET", p.config.BaseURL+endpoint, nil)
    if err != nil {
        return fmt.Errorf("build request: %w", err)
    }
    req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

    resp, err := p.client.Do(req)
    if err != nil {
        return fmt.Errorf("do request: %w", err)
    }
    defer resp.Body.Close()

    switch resp.StatusCode {
    case http.StatusOK:
    case http.StatusUnauthorized, http.StatusForbidden:
        return adapter.ErrAuthExpired
    case http.StatusTooManyRequests:
        return adapter.ErrRateLimited
    default:
        return fmt.Errorf("%w: HTTP %d", adapter.ErrProviderUnavailable, resp.StatusCode)
    }

    if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
        return fmt.Errorf("decode response: %w", err)
    }
    return nil
}
```

### Polling vs. Webhooks

- **Polling**: `IngestionService` calls adapter methods on a ticker interval (default 5 min)
- **Webhooks**: Provider pushes updates; adapter exposes an HTTP handler registered on a separate route

The ingestion service handles all polling scheduling and backoff — adapters only implement data-fetching methods.

### Rate Limiting

The `IngestionService` handles backoff on `ErrRateLimited`. Adapters return the sentinel error; the service decides the retry strategy:

```go
// internal/service/ingestion_service.go
if errors.Is(err, adapter.ErrRateLimited) {
    s.backoff.increase()
    slog.Warn("ingestion: rate limited", "provider", s.providerType, "retry_in", s.backoff.current())
    return
}
```

## Provider Credentials

Provider API keys are loaded from environment variables. See `.env.example` for the full list. Credentials are never stored in the database.

```bash
# .env
ENPHASE_API_KEY=your_enphase_key
ENPHASE_SYSTEM_ID=your_system_id
```

## API Provider Resources

- [Enphase API Documentation](https://developer.enphase.com/)
- [Tesla API (Powerwall)](https://developer.tesla.com/)
- [SolarEdge API](https://www.solaredge.com/en/developers/documentation)
- [Fronius API](https://www.fronius.com/en-us/usa/solar-energy/installers-partners/system-partners/fronius-datcom-api)

## Troubleshooting

### Authentication Failures

The adapter returns `adapter.ErrAuthExpired`. Check:
- API key is correct in `.env`
- Token hasn't expired (OAuth2 providers require periodic re-authentication)
- Provider credentials match the configured system/site ID

### Data Parsing Errors

Errors are wrapped with context: `"newprovider: decode response: ..."`. Check:
- Provider API version hasn't changed response format
- Test with `MockSystemStatus()` to confirm adapter code is correct

### Rate Limiting

The ingestion service backs off automatically on `adapter.ErrRateLimited`. Check:
- Polling interval is not too aggressive for the provider's limits
- Multiple instances are not polling the same provider simultaneously
