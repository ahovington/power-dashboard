package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/power-dashboard/internal/model"
)

//go:generate mockgen -destination=../service/mock_power_servicer.go -package=service -mock_names PowerServicer=MockPowerServicer github.com/yourusername/power-dashboard/internal/api PowerServicer

// PowerServicer is the interface the handler uses to query power data.
// The concrete *service.PowerService satisfies this.
type PowerServicer interface {
	GetCurrentStatus(ctx context.Context, deviceID uuid.UUID) (*model.PowerReading, error)
	GetHistory(ctx context.Context, deviceID uuid.UUID, interval string, start, end time.Time) ([]*model.PowerReading, error)
}

type Handler struct {
	power PowerServicer
	hub   *Hub
	db    *pgxpool.Pool
}

func NewHandler(power PowerServicer, hub *Hub, db *pgxpool.Pool) *Handler {
	return &Handler{power: power, hub: hub, db: db}
}

func (h *Handler) GetCurrentStatus(w http.ResponseWriter, r *http.Request) {
	deviceID, err := uuid.Parse(r.URL.Query().Get("device_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid device_id")
		return
	}

	reading, err := h.power.GetCurrentStatus(r.Context(), deviceID)
	if err != nil {
		slog.Error("handler: get current status", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if reading == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "no data"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"device_id":      reading.DeviceID,
		"timestamp":      reading.ReadingTimestamp,
		"power_produced": reading.PowerProduced,
		"power_consumed": reading.PowerConsumed,
		"power_net":      reading.PowerNet(),
	})
}

func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
	deviceID, err := uuid.Parse(r.URL.Query().Get("device_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid device_id")
		return
	}

	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "hour"
	}

	start, err := time.Parse(time.RFC3339, r.URL.Query().Get("start"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start: use RFC3339")
		return
	}

	end, err := time.Parse(time.RFC3339, r.URL.Query().Get("end"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end: use RFC3339")
		return
	}

	readings, err := h.power.GetHistory(r.Context(), deviceID, interval, start, end)
	if err != nil {
		slog.Error("handler: get history", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, readings)
}

func (h *Handler) ServeEvents(w http.ResponseWriter, r *http.Request) {
	h.hub.ServeSSE(w, r)
}

// Ready handles GET /ready.
// Returns 503 if the DB pool cannot be pinged — used by Docker healthcheck.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	if err := h.db.Ping(r.Context()); err != nil {
		slog.Error("readiness: db ping failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("handler: encode response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
