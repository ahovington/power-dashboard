package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ahovingtonpower-dashboard/internal/api"
	"github.com/ahovingtonpower-dashboard/internal/model"
	"github.com/ahovingtonpower-dashboard/internal/service"
)

func TestGetCurrentStatus_ValidDevice(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deviceID := uuid.New()
	mockSvc := service.NewMockPowerServicer(ctrl)
	mockSvc.EXPECT().GetCurrentStatus(gomock.Any(), deviceID).Return(&model.PowerReading{
		PowerProduced: 5000, PowerConsumed: 3000,
	}, nil)

	h := api.NewHandler(mockSvc, api.NewHub(), nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/power/status?device_id="+deviceID.String(), nil)
	w := httptest.NewRecorder()
	h.GetCurrentStatus(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(5000), resp["power_produced"])
	assert.Equal(t, float64(2000), resp["power_net"])
}

func TestGetCurrentStatus_InvalidDeviceID(t *testing.T) {
	h := api.NewHandler(nil, nil, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/power/status?device_id=not-a-uuid", nil)
	w := httptest.NewRecorder()
	h.GetCurrentStatus(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetCurrentStatus_NoData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := service.NewMockPowerServicer(ctrl)
	mockSvc.EXPECT().GetCurrentStatus(gomock.Any(), gomock.Any()).Return(nil, nil)

	h := api.NewHandler(mockSvc, api.NewHub(), nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/power/status?device_id="+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	h.GetCurrentStatus(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetHistory_ValidRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deviceID := uuid.New()
	mockSvc := service.NewMockPowerServicer(ctrl)
	mockSvc.EXPECT().GetHistory(gomock.Any(), deviceID, "hour", gomock.Any(), gomock.Any()).
		Return([]*model.PowerReading{{PowerProduced: 3000}}, nil)

	h := api.NewHandler(mockSvc, api.NewHub(), nil)
	url := fmt.Sprintf("/api/v1/power/history?device_id=%s&interval=hour&start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z", deviceID)
	r := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	h.GetHistory(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetHistory_InvalidInterval(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := service.NewMockPowerServicer(ctrl)
	mockSvc.EXPECT().GetHistory(gomock.Any(), gomock.Any(), "minute", gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("invalid interval"))

	h := api.NewHandler(mockSvc, api.NewHub(), nil)
	url := fmt.Sprintf("/api/v1/power/history?device_id=%s&interval=minute&start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z", uuid.New())
	r := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	h.GetHistory(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// Ensure unused time import is referenced.
var _ = time.Now
