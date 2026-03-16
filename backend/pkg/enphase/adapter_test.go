package enphase_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/power-dashboard/pkg/adapter"
	"github.com/yourusername/power-dashboard/pkg/enphase"
)

func TestGetSystemStatus_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(enphase.MockSystemStatusResponse())
	}))
	defer srv.Close()

	a := enphase.NewAdapter(enphase.Config{APIKey: "test-key", BaseURL: srv.URL})
	status, err := a.GetSystemStatus(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 5000, status.PowerProduced)
	assert.Equal(t, 3000, status.PowerConsumed)
}

func TestGetSystemStatus_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	a := enphase.NewAdapter(enphase.Config{APIKey: "test", BaseURL: srv.URL})
	_, err := a.GetSystemStatus(context.Background())

	assert.ErrorIs(t, err, adapter.ErrRateLimited)
}

func TestGetSystemStatus_AuthExpired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := enphase.NewAdapter(enphase.Config{APIKey: "expired", BaseURL: srv.URL})
	_, err := a.GetSystemStatus(context.Background())

	assert.ErrorIs(t, err, adapter.ErrAuthExpired)
}

func TestGetSystemStatus_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := enphase.NewAdapter(enphase.Config{APIKey: "test", BaseURL: srv.URL})
	_, err := a.GetSystemStatus(context.Background())

	assert.ErrorIs(t, err, adapter.ErrProviderUnavailable)
}

func TestGetSystemStatus_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json {{{"))
	}))
	defer srv.Close()

	a := enphase.NewAdapter(enphase.Config{APIKey: "test", BaseURL: srv.URL})
	_, err := a.GetSystemStatus(context.Background())

	assert.Error(t, err)
	assert.NotErrorIs(t, err, adapter.ErrRateLimited)
}

func TestGetSystemStatus_NetworkTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // hang until client gives up
	}))
	defer srv.Close()

	a := enphase.NewAdapter(enphase.Config{
		APIKey:         "test",
		BaseURL:        srv.URL,
		RequestTimeout: 50 * time.Millisecond,
	})
	_, err := a.GetSystemStatus(context.Background())

	assert.Error(t, err)
}
