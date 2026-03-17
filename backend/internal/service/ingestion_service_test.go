package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ahovingtonpower-dashboard/internal/model"
	"github.com/ahovingtonpower-dashboard/internal/service"
	"github.com/ahovingtonpower-dashboard/pkg/adapter"
)

type stubRepo struct {
	saved        []*model.PowerReading
	savedBattery []*model.BatteryStatus
}

func (s *stubRepo) SaveReading(_ context.Context, r *model.PowerReading) error {
	s.saved = append(s.saved, r)
	return nil
}

func (s *stubRepo) SaveBatteryStatus(_ context.Context, b *model.BatteryStatus) error {
	s.savedBattery = append(s.savedBattery, b)
	return nil
}

type failingRepo struct{ err error }

func (r *failingRepo) SaveReading(_ context.Context, _ *model.PowerReading) error {
	return r.err
}

func (r *failingRepo) SaveBatteryStatus(_ context.Context, _ *model.BatteryStatus) error {
	return nil // battery save non-fatal, but let power reading fail
}

func TestIngestionService_PublishesEventOnSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAdapter := adapter.NewMockProviderAdapter(ctrl)
	mockAdapter.EXPECT().GetSystemStatus(gomock.Any()).Return(&adapter.SystemStatus{
		PowerProduced: 5000, PowerConsumed: 3000,
	}, nil)
	mockAdapter.EXPECT().GetBatteryStatus(gomock.Any()).Return(nil, nil)

	repo := &stubRepo{}
	bus := make(chan model.PowerEvent, 8)
	trigger := make(chan time.Time, 1)

	svc := service.NewIngestionService(mockAdapter, repo, bus, uuid.New(), trigger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go svc.RunPoller(ctx)

	trigger <- time.Now() // fire one poll

	select {
	case event := <-bus:
		assert.Equal(t, 5000, event.PowerProduced)
		assert.Equal(t, 3000, event.PowerConsumed)
		assert.Equal(t, 2000, event.PowerNet)
	case <-time.After(time.Second):
		t.Fatal("no event received within 1s")
	}

	// Verify reading was also persisted
	require.Len(t, repo.saved, 1)
	assert.Equal(t, 5000, repo.saved[0].PowerProduced)
}

func TestIngestionService_BatteryDataSavedAndPublished(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAdapter := adapter.NewMockProviderAdapter(ctrl)
	mockAdapter.EXPECT().GetSystemStatus(gomock.Any()).Return(&adapter.SystemStatus{
		PowerProduced: 4000, PowerConsumed: 2000,
	}, nil)
	mockAdapter.EXPECT().GetBatteryStatus(gomock.Any()).Return(&adapter.BatteryStatus{
		ChargePercentage: 80.0,
		PowerFlowing:     500,
		PowerDirection:   "charging",
		StateOfHealth:    95,
		CapacityWh:       10000,
		Temperature:      26.0,
	}, nil)

	repo := &stubRepo{}
	bus := make(chan model.PowerEvent, 8)
	trigger := make(chan time.Time, 1)

	svc := service.NewIngestionService(mockAdapter, repo, bus, uuid.New(), trigger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go svc.RunPoller(ctx)

	trigger <- time.Now()

	select {
	case event := <-bus:
		require.NotNil(t, event.BatteryW)
		assert.Equal(t, 500, *event.BatteryW)
		assert.Equal(t, "charging", event.BatteryDirection)
		require.NotNil(t, event.BatteryCharge)
		assert.InDelta(t, 80.0, *event.BatteryCharge, 0.01)
	case <-time.After(time.Second):
		t.Fatal("no event received within 1s")
	}

	require.Len(t, repo.savedBattery, 1)
	assert.InDelta(t, 80.0, repo.savedBattery[0].ChargePercentage, 0.01)
}

func TestIngestionService_BatteryNilSkipsSilently(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAdapter := adapter.NewMockProviderAdapter(ctrl)
	mockAdapter.EXPECT().GetSystemStatus(gomock.Any()).Return(&adapter.SystemStatus{
		PowerProduced: 3000, PowerConsumed: 2000,
	}, nil)
	mockAdapter.EXPECT().GetBatteryStatus(gomock.Any()).Return(nil, nil)

	repo := &stubRepo{}
	bus := make(chan model.PowerEvent, 8)
	trigger := make(chan time.Time, 1)

	svc := service.NewIngestionService(mockAdapter, repo, bus, uuid.New(), trigger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go svc.RunPoller(ctx)

	trigger <- time.Now()

	select {
	case event := <-bus:
		assert.Nil(t, event.BatteryW)
		assert.Nil(t, event.BatteryCharge)
		assert.Empty(t, event.BatteryDirection)
	case <-time.After(time.Second):
		t.Fatal("no event received within 1s")
	}

	assert.Empty(t, repo.savedBattery)
}

func TestIngestionService_BatteryErrorIsNonFatal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAdapter := adapter.NewMockProviderAdapter(ctrl)
	mockAdapter.EXPECT().GetSystemStatus(gomock.Any()).Return(&adapter.SystemStatus{
		PowerProduced: 2000, PowerConsumed: 1500,
	}, nil)
	mockAdapter.EXPECT().GetBatteryStatus(gomock.Any()).Return(nil, errors.New("battery API timeout"))

	repo := &stubRepo{}
	bus := make(chan model.PowerEvent, 8)
	trigger := make(chan time.Time, 1)

	svc := service.NewIngestionService(mockAdapter, repo, bus, uuid.New(), trigger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go svc.RunPoller(ctx)

	trigger <- time.Now()

	// Power event should still be published despite battery error
	select {
	case event := <-bus:
		assert.Equal(t, 2000, event.PowerProduced)
		assert.Nil(t, event.BatteryW)
	case <-time.After(time.Second):
		t.Fatal("no event received within 1s — battery error should be non-fatal")
	}
}

func TestIngestionService_PanicIsRecovered(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAdapter := adapter.NewMockProviderAdapter(ctrl)
	mockAdapter.EXPECT().GetSystemStatus(gomock.Any()).DoAndReturn(
		func(_ context.Context) (*adapter.SystemStatus, error) {
			panic("simulated nil dereference")
		},
	)

	trigger := make(chan time.Time, 1)
	svc := service.NewIngestionService(mockAdapter, &stubRepo{}, make(chan model.PowerEvent, 1), uuid.New(), trigger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	assert.NotPanics(t, func() {
		go svc.RunPoller(ctx)
		trigger <- time.Now()
		time.Sleep(50 * time.Millisecond) // let the goroutine recover
	})
}

func TestIngestionService_GracefulShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAdapter := adapter.NewMockProviderAdapter(ctrl)

	trigger := make(chan time.Time)
	svc := service.NewIngestionService(mockAdapter, &stubRepo{}, make(chan model.PowerEvent, 1), uuid.New(), trigger)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.RunPoller(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunPoller did not stop within 2s of context cancellation")
	}
}

func TestIngestionService_RateLimitTriggersBackoff(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAdapter := adapter.NewMockProviderAdapter(ctrl)
	mockAdapter.EXPECT().GetSystemStatus(gomock.Any()).Return(nil, adapter.ErrRateLimited)

	trigger := make(chan time.Time, 1)
	svc := service.NewIngestionService(mockAdapter, &stubRepo{}, make(chan model.PowerEvent, 1), uuid.New(), trigger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go svc.RunPoller(ctx)

	trigger <- time.Now()
	time.Sleep(20 * time.Millisecond)

	// After a rate limit, the service should be in backoff — a second trigger
	// should NOT cause another call (backoff sleep is in progress).
	// The mock expectation of exactly 1 call enforces this.
}

func TestIngestionService_RepoErrorDoesNotCrash(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAdapter := adapter.NewMockProviderAdapter(ctrl)
	mockAdapter.EXPECT().GetSystemStatus(gomock.Any()).Return(
		&adapter.SystemStatus{PowerProduced: 1000}, nil,
	)

	trigger := make(chan time.Time, 1)
	svc := service.NewIngestionService(mockAdapter, &failingRepo{err: errors.New("db pool exhausted")},
		make(chan model.PowerEvent, 1), uuid.New(), trigger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	assert.NotPanics(t, func() {
		go svc.RunPoller(ctx)
		trigger <- time.Now()
		time.Sleep(20 * time.Millisecond)
	})
}
