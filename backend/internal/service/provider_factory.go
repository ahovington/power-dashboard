package service

import (
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/power-dashboard/pkg/adapter"
	"github.com/yourusername/power-dashboard/pkg/enphase"
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
func BuildProviders(trigger <-chan time.Time) []ProviderIngestionConfig {
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
