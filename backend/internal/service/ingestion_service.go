package service

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/power-dashboard/internal/model"
	"github.com/yourusername/power-dashboard/pkg/adapter"
)

type ReadingWriter interface {
	SaveReading(ctx context.Context, r *model.PowerReading) error
}

// IngestionService polls a ProviderAdapter on every tick, persists readings,
// and publishes PowerEvents to the SSE hub via eventBus.
//
//	ticker.C (or test channel) ──► pollSafe ──► pollOnce ──► SaveReading
//	                                                      └──► eventBus
//
// Panics in pollOnce are recovered. Errors trigger exponential backoff.
// Backoff sequence: 1s → 2s → 4s → ... → 5min (factor starts at 0).
type IngestionService struct {
	adapter  adapter.ProviderAdapter
	repo     ReadingWriter
	eventBus chan<- model.PowerEvent
	deviceID uuid.UUID
	trigger  <-chan time.Time // time.NewTicker(interval).C in production; injected in tests
}

func NewIngestionService(
	a adapter.ProviderAdapter,
	repo ReadingWriter,
	eventBus chan<- model.PowerEvent,
	deviceID uuid.UUID,
	trigger <-chan time.Time,
) *IngestionService {
	return &IngestionService{adapter: a, repo: repo, eventBus: eventBus, deviceID: deviceID, trigger: trigger}
}

func (s *IngestionService) RunPoller(ctx context.Context) {
	slog.Info("ingestion: starting poller", "device_id", s.deviceID)
	b := newBackoff(time.Second, 5*time.Minute)
	for {
		select {
		case <-ctx.Done():
			slog.Info("ingestion: poller stopped", "device_id", s.deviceID)
			return
		case <-s.trigger:
			s.pollSafe(ctx, b)
		}
	}
}

func (s *IngestionService) pollSafe(ctx context.Context, b *backoff) {
	start := time.Now()
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("ingestion: panic recovered",
				"device_id", s.deviceID, "panic", rec, "retry_in", b.current())
			time.Sleep(b.increase())
		}
	}()

	if err := s.pollOnce(ctx); err != nil {
		if errors.Is(err, adapter.ErrRateLimited) {
			slog.Warn("ingestion: rate limited",
				"device_id", s.deviceID, "retry_in", b.current())
		} else {
			slog.Error("ingestion: poll failed",
				"device_id", s.deviceID, "error", err, "retry_in", b.current())
		}
		time.Sleep(b.increase())
		return
	}

	b.reset()
	slog.Info("ingestion: cycle complete",
		"device_id", s.deviceID, "duration_ms", time.Since(start).Milliseconds())
}

func (s *IngestionService) pollOnce(ctx context.Context) error {
	status, err := s.adapter.GetSystemStatus(ctx)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	if err := s.repo.SaveReading(ctx, &model.PowerReading{
		DeviceID:         s.deviceID,
		ReadingTimestamp: now,
		PowerProduced:    status.PowerProduced,
		PowerConsumed:    status.PowerConsumed,
	}); err != nil {
		return err
	}

	event := model.PowerEvent{
		DeviceID:      s.deviceID,
		Timestamp:     now,
		PowerProduced: status.PowerProduced,
		PowerConsumed: status.PowerConsumed,
		PowerNet:      status.PowerProduced - status.PowerConsumed,
	}
	select {
	case s.eventBus <- event:
	default:
		slog.Warn("ingestion: event bus full, dropping event", "device_id", s.deviceID)
	}
	return nil
}

// backoff implements exponential backoff with a cap.
// Factor starts at 0 so the first sleep is 2^0 * min = 1 * min.
// Sequence with min=1s: 1s → 2s → 4s → 8s → ... → 5min.
type backoff struct {
	min, max, cur time.Duration
	factor        int
}

func newBackoff(min, max time.Duration) *backoff {
	return &backoff{min: min, max: max, cur: min, factor: 0}
}

func (b *backoff) current() time.Duration { return b.cur }

func (b *backoff) reset() { b.cur = b.min; b.factor = 0 }

func (b *backoff) increase() time.Duration {
	next := time.Duration(math.Pow(2, float64(b.factor))) * b.min
	if next > b.max {
		next = b.max
	}
	b.cur = next
	b.factor++
	return next
}
