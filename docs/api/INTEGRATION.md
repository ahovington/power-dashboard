# API Integration Guide

## Overview

This guide explains how to integrate new energy API providers into the Power Monitor system.

## Architecture

The system uses the **Adapter Pattern** to abstract different provider implementations:

```
ProviderAdapter Interface
    ↑
    ├── EnphaseAdapter
    ├── TeslaAdapter (Future)
    └── NewProviderAdapter
```

## Creating a New Provider Adapter

### Step 1: Define the Adapter Interface

All providers must implement the `ProviderAdapter` interface:

```go
// pkg/adapter/provider_adapter.go
package adapter

import (
    "context"
    "time"
)

type SystemStatus struct {
    ID                string
    Name              string
    Status            string
    PowerProduced     int
    PowerConsumed     int
    PowerNet          int
    BatteryCharge     int
}

type PowerMetrics struct {
    Timestamp         time.Time
    PowerProduced     int
    PowerConsumed     int
    PowerNet          int
    Frequency         float64
    VoltagePhaseA     float64
    VoltagePhaseB     float64
    VoltagePhaseC     float64
}

type ProviderAdapter interface {
    // Authenticate with the provider API
    Authenticate(ctx context.Context, credentials interface{}) error
    
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

### Step 2: Create Provider Package

Create a new directory for your provider:

```bash
mkdir -p backend/pkg/newprovider
```

### Step 3: Implement the Adapter

```go
// pkg/newprovider/adapter.go
package newprovider

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
    
    "yourmodule/internal/model"
    "yourmodule/pkg/adapter"
)

type NewProviderAdapter struct {
    apiKey    string
    baseURL   string
    httpClient *http.Client
}

func NewAdapter(apiKey string) *NewProviderAdapter {
    return &NewProviderAdapter{
        apiKey: apiKey,
        baseURL: "https://api.newprovider.com/v1",
        httpClient: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}

func (a *NewProviderAdapter) Authenticate(ctx context.Context, credentials interface{}) error {
    // Implement authentication logic
    return nil
}

func (a *NewProviderAdapter) GetSystemStatus(ctx context.Context) (*adapter.SystemStatus, error) {
    // Make API request to provider
    req, err := http.NewRequestWithContext(ctx, "GET", a.baseURL+"/system/status", nil)
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("Authorization", "Bearer "+a.apiKey)
    
    resp, err := a.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var statusResp SystemStatusResponse
    if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
        return nil, err
    }
    
    // Transform to common format
    return &adapter.SystemStatus{
        ID: statusResp.SystemID,
        Name: statusResp.Name,
        Status: statusResp.State,
        PowerProduced: statusResp.CurrentProduction,
        PowerConsumed: statusResp.CurrentConsumption,
        PowerNet: statusResp.CurrentProduction - statusResp.CurrentConsumption,
    }, nil
}

// Implement other interface methods...
```

### Step 4: Create Type Definitions

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
    Timestamp string `json:"timestamp"`
    Production int  `json:"production"`
    Consumption int `json:"consumption"`
    Frequency float64 `json:"frequency"`
}
```

### Step 5: Create Mock Responses

```go
// pkg/newprovider/mock_responses.go
package newprovider

func MockSystemStatus() *SystemStatusResponse {
    return &SystemStatusResponse{
        SystemID: "mock-system-123",
        Name: "Test System",
        State: "normal",
        CurrentProduction: 5000,
        CurrentConsumption: 3000,
    }
}
```

### Step 6: Register Adapter in Service

```go
// internal/service/provider_factory.go
package service

import (
    "fmt"
    "yourmodule/pkg/adapter"
    "yourmodule/pkg/enphase"
    "yourmodule/pkg/newprovider"
)

func CreateAdapter(providerType string, config interface{}) (adapter.ProviderAdapter, error) {
    switch providerType {
    case "enphase":
        cfg := config.(map[string]string)
        return enphase.NewAdapter(cfg["api_key"]), nil
    case "newprovider":
        cfg := config.(map[string]string)
        return newprovider.NewAdapter(cfg["api_key"]), nil
    default:
        return nil, fmt.Errorf("unsupported provider: %s", providerType)
    }
}
```

### Step 7: Add Tests

```go
// pkg/newprovider/adapter_test.go
package newprovider

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestGetSystemStatus(t *testing.T) {
    adapter := NewAdapter("test-key")
    
    status, err := adapter.GetSystemStatus(context.Background())
    
    assert.NoError(t, err)
    assert.NotNil(t, status)
    assert.Greater(t, status.PowerProduced, 0)
}
```

## Integration Checklist

- [ ] Adapter implements `ProviderAdapter` interface
- [ ] API client handles authentication
- [ ] Response parsing with error handling
- [ ] Type conversions to common format
- [ ] Mock responses for testing
- [ ] Unit tests for adapter
- [ ] Integration tests with service
- [ ] Documentation of API endpoints
- [ ] Error handling for rate limits
- [ ] Proper logging

## Common Provider API Patterns

### REST API

```go
type RESTProvider struct {
    baseURL string
    client  *http.Client
}

func (p *RESTProvider) makeRequest(method, endpoint string) (*Response, error) {
    // Implementation
}
```

### Polling vs. Webhooks

- **Polling**: Adapter queries provider API on interval
- **Webhooks**: Provider pushes updates to adapter

```go
// Polling implementation
func (a *Adapter) Poll(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    for range ticker.C {
        a.GetSystemStatus(ctx)
    }
}
```

### Rate Limiting

Respect provider rate limits:

```go
import "golang.org/x/time/rate"

limiter := rate.NewLimiter(rate.Every(time.Second), 10) // 10 requests per second

if !limiter.Allow() {
    return fmt.Errorf("rate limit exceeded")
}
```

## Best Practices

1. **Error Handling**: Return meaningful errors
2. **Timeouts**: Set reasonable request timeouts
3. **Logging**: Log API interactions for debugging
4. **Caching**: Cache responses to reduce API calls
5. **Testing**: Use mock responses instead of real API calls
6. **Documentation**: Document API credentials needed
7. **Secrets**: Never commit API keys
8. **Versioning**: Handle API version changes

## API Provider Resources

- [Enphase API Documentation](https://developer.enphase.com/)
- [Tesla API (Powerwall)](https://developer.tesla.com/)
- [SolarEdge API](https://www.solaredge.com/en/developers/documentation)
- [Fronius API](https://www.fronius.com/en-us/usa/solar-energy/installers-partners/system-partners/fronius-datcom-api)

## Troubleshooting

### Authentication Failures

Check:
- API key is correct
- Token hasn't expired
- Provider credentials are in environment variables

### Data Parsing Errors

Test with:
- Mock responses that match provider format
- JSON unmarshaling tests
- Sample API responses from provider

### Rate Limiting

Implement:
- Request throttling
- Exponential backoff for retries
- Local caching