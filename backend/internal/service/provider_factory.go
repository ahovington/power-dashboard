package service

import (
	"log/slog"
	"time"

	"github.com/ahovingtonpower-dashboard/internal/config"
	"github.com/ahovingtonpower-dashboard/pkg/adapter"
	"github.com/ahovingtonpower-dashboard/pkg/enphase"
	"github.com/ahovingtonpower-dashboard/pkg/fake"
	"github.com/google/uuid"
)

// ProviderIngestionConfig holds what's needed to start one IngestionService.
type ProviderIngestionConfig struct {
	Adapter  adapter.ProviderAdapter
	DeviceID uuid.UUID
	Trigger  <-chan time.Time
}

// BuildProviders returns one config per configured provider, based on cfg.
// Enphase is enabled when ENPHASE_API_KEY is set; the fake provider is enabled
// when FAKE_PROVIDER=true. Future providers follow the same pattern.
func BuildProviders(cfg *config.Config, trigger <-chan time.Time) []ProviderIngestionConfig {
	var providers []ProviderIngestionConfig

	if cfg.EnphaseAPIKey != "" {
		slog.Info("provider: enphase configured")
		providers = append(providers, ProviderIngestionConfig{
			Adapter: enphase.NewAdapter(enphase.Config{
				APIKey:       cfg.EnphaseAPIKey,
				SystemID:     cfg.EnphaseSystemID,
				ClientID:     cfg.EnphaseClientID,
				ClientSecret: cfg.EnphaseClientSecret,
				AccessToken:  cfg.EnphaseAccessToken,
				RefreshToken: cfg.EnphaseRefreshToken,
			}),
			DeviceID: uuid.New(), // TODO: load from devices table once DeviceRepository is wired
			Trigger:  trigger,
		})
	}

	if cfg.FakeProvider {
		slog.Info("provider: fake configured (synthetic data)", "seed", cfg.FakeSeed)
		providers = append(providers, ProviderIngestionConfig{
			Adapter:  fake.NewAdapter(fake.FakeConfig{Seed: cfg.FakeSeed, TimeZone: cfg.FakeTimezone}),
			DeviceID: fake.FakeDeviceID,
			Trigger:  trigger,
		})
	}

	if len(providers) == 0 {
		slog.Warn("provider_factory: no providers configured — ingestion will not run")
	}

	return providers
}

