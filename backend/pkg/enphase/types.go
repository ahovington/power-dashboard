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
