package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"

	"github.com/ahovingtonpower-dashboard/internal/model"
)

const sseClientBuffer = 16

// Hub fans out PowerEvents to all connected SSE clients.
//
//	IngestionService ──► eventBus ──► Hub.Broadcast(event)
//	                                       │  fan-out
//	                           ┌───────────┼───────────┐
//	                           ▼           ▼           ▼
//	                      client A    client B    client N
//	                     (buffered)  (buffered)  (buffered)
//
// Slow clients are dropped (buffer full) rather than blocking the broadcast loop.
type Hub struct {
	subscribe   chan chan model.PowerEvent
	unsubscribe chan chan model.PowerEvent
	broadcast   chan model.PowerEvent
	connected   atomic.Int64
}

func NewHub() *Hub {
	return &Hub{
		// Unbuffered subscribe/unsubscribe: callers block until Run() processes the
		// request, guaranteeing the client map is updated before returning. This
		// prevents races where Broadcast() fires before a subscription is registered.
		subscribe:   make(chan chan model.PowerEvent),
		unsubscribe: make(chan chan model.PowerEvent),
		broadcast:   make(chan model.PowerEvent, 32),
	}
}

func (h *Hub) Run(ctx context.Context) {
	clients := make(map[chan model.PowerEvent]struct{})
	for {
		select {
		case <-ctx.Done():
			for ch := range clients {
				close(ch)
			}
			return
		case ch := <-h.subscribe:
			clients[ch] = struct{}{}
			h.connected.Add(1)
		case ch := <-h.unsubscribe:
			if _, ok := clients[ch]; ok {
				delete(clients, ch)
				close(ch)
				h.connected.Add(-1)
			}
		case event := <-h.broadcast:
			for ch := range clients {
				select {
				case ch <- event:
				default:
					slog.Warn("sse: dropping event for slow client")
				}
			}
		}
	}
}

func (h *Hub) Subscribe() chan model.PowerEvent {
	ch := make(chan model.PowerEvent, sseClientBuffer)
	h.subscribe <- ch
	return ch
}

func (h *Hub) Unsubscribe(ch chan model.PowerEvent) { h.unsubscribe <- ch }

func (h *Hub) Broadcast(event model.PowerEvent) { h.broadcast <- event }

func (h *Hub) ConnectedClients() int64 { return h.connected.Load() }

// ServeSSE handles GET /api/v1/events.
// No write timeout — the connection is intentionally long-lived.
// Ensure your reverse proxy (nginx) sets an appropriate proxy_read_timeout.
func (h *Hub) ServeSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := h.Subscribe()
	defer h.Unsubscribe(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				slog.Error("sse: marshal event", "error", err)
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
