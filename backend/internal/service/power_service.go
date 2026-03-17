package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ahovingtonpower-dashboard/internal/model"
	"github.com/google/uuid"
)

//go:generate mockgen -source=power_service.go -destination=mock_reading_querier.go -package=service

type ReadingQuerier interface {
	GetLatestReadings(ctx context.Context, deviceID uuid.UUID, limit int) ([]*model.PowerReading, error)
	GetAggregatedReadings(ctx context.Context, deviceID uuid.UUID, interval string, start, end time.Time) ([]*model.PowerReading, error)
	GetLatestBatteryStatus(ctx context.Context, deviceID uuid.UUID) (*model.BatteryStatus, error)
}

type PowerService struct{ repo ReadingQuerier }

func NewPowerService(repo ReadingQuerier) *PowerService { return &PowerService{repo: repo} }

func (s *PowerService) GetCurrentStatus(ctx context.Context, deviceID uuid.UUID) (*model.PowerReading, error) {
	readings, err := s.repo.GetLatestReadings(ctx, deviceID, 1)
	if err != nil {
		return nil, fmt.Errorf("power service: get current status: %w", err)
	}
	if len(readings) == 0 {
		return nil, nil
	}
	return readings[0], nil
}

func (s *PowerService) GetHistory(ctx context.Context, deviceID uuid.UUID, interval string, start, end time.Time) ([]*model.PowerReading, error) {
	valid := map[string]bool{"hour": true, "day": true, "week": true, "month": true}
	if !valid[interval] {
		return nil, fmt.Errorf("power service: invalid interval %q (must be hour|day|week|month)", interval)
	}
	return s.repo.GetAggregatedReadings(ctx, deviceID, interval, start, end)
}

func (s *PowerService) GetLatestBatteryStatus(ctx context.Context, deviceID uuid.UUID) (*model.BatteryStatus, error) {
	return s.repo.GetLatestBatteryStatus(ctx, deviceID)
}
