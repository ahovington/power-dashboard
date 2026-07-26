package enphase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/ahovingtonpower-dashboard/pkg/adapter"
)

const defaultBaseURL = "https://api.enphaseenergy.com/api/v4"

type Adapter struct {
	cfg    Config
	client *http.Client
	mu     sync.Mutex // guards cfg.AccessToken during refresh
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

// get executes an authenticated GET request. On 401 it attempts a token
// refresh once (if a refresh token is configured) and retries.
func (a *Adapter) get(ctx context.Context, path string, out interface{}) error {
	if err := a.doGet(ctx, path, out); err != adapter.ErrAuthExpired {
		return err
	}
	// 401 — try refreshing the access token then retry once
	if err := a.refresh(); err != nil {
		slog.Warn("enphase: token refresh failed, returning auth error", "error", err)
		return adapter.ErrAuthExpired
	}
	return a.doGet(ctx, path, out)
}

func (a *Adapter) doGet(ctx context.Context, path string, out interface{}) error {
	a.mu.Lock()
	token := a.cfg.AccessToken
	apiKey := a.cfg.APIKey
	a.mu.Unlock()

	reqURL := a.cfg.BaseURL + path + "?key=" + apiKey
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("enphase: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

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

// refresh exchanges the stored refresh token for a new access token.
func (a *Adapter) refresh() error {
	a.mu.Lock()
	refreshToken := a.cfg.RefreshToken
	clientID := a.cfg.ClientID
	clientSecret := a.cfg.ClientSecret
	a.mu.Unlock()

	if refreshToken == "" || clientID == "" || clientSecret == "" {
		return fmt.Errorf("enphase: no refresh token or client credentials configured")
	}

	t, err := RefreshAccessToken(clientID, clientSecret, refreshToken)
	if err != nil {
		return err
	}

	a.mu.Lock()
	a.cfg.AccessToken = t.AccessToken
	if t.RefreshToken != "" {
		a.cfg.RefreshToken = t.RefreshToken
	}
	a.mu.Unlock()

	slog.Info("enphase: access token refreshed")
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
