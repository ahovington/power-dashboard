package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/yourusername/power-dashboard/internal/model"
	"github.com/yourusername/power-dashboard/internal/service"
)

func TestPowerService_GetCurrentStatus_ReturnsLatest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reading := &model.PowerReading{PowerProduced: 5000, PowerConsumed: 3000}
	mockRepo := service.NewMockReadingQuerier(ctrl)
	mockRepo.EXPECT().GetLatestReadings(gomock.Any(), gomock.Any(), 1).Return([]*model.PowerReading{reading}, nil)

	svc := service.NewPowerService(mockRepo)
	result, err := svc.GetCurrentStatus(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.Equal(t, 5000, result.PowerProduced)
	assert.Equal(t, 2000, result.PowerNet())
}

func TestPowerService_GetCurrentStatus_NoDataReturnsNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := service.NewMockReadingQuerier(ctrl)
	mockRepo.EXPECT().GetLatestReadings(gomock.Any(), gomock.Any(), 1).Return(nil, nil)

	svc := service.NewPowerService(mockRepo)
	result, err := svc.GetCurrentStatus(context.Background(), uuid.New())

	assert.NoError(t, err)
	assert.Nil(t, result, "no data yet is not an error")
}

func TestPowerService_GetHistory_RejectsInvalidInterval(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := service.NewPowerService(service.NewMockReadingQuerier(ctrl))
	_, err := svc.GetHistory(context.Background(), uuid.New(), "minute", time.Now(), time.Now())

	assert.Error(t, err)
}

func TestPowerService_GetHistory_ValidIntervals(t *testing.T) {
	for _, interval := range []string{"hour", "day", "week", "month"} {
		t.Run(interval, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := service.NewMockReadingQuerier(ctrl)
			mockRepo.EXPECT().
				GetAggregatedReadings(gomock.Any(), gomock.Any(), interval, gomock.Any(), gomock.Any()).
				Return([]*model.PowerReading{}, nil)

			svc := service.NewPowerService(mockRepo)
			_, err := svc.GetHistory(context.Background(), uuid.New(), interval, time.Now(), time.Now())
			assert.NoError(t, err)
		})
	}
}
