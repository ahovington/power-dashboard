package api_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ahovingtonpower-dashboard/internal/api"
	"github.com/ahovingtonpower-dashboard/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHub_SingleClientReceivesEvent(t *testing.T) {
	hub := api.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	hub.Broadcast(model.PowerEvent{PowerProduced: 5000})

	select {
	case got := <-ch:
		assert.Equal(t, 5000, got.PowerProduced)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestHub_MultipleClientsAllReceive(t *testing.T) {
	hub := api.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	ch1 := hub.Subscribe()
	ch2 := hub.Subscribe()
	defer hub.Unsubscribe(ch1)
	defer hub.Unsubscribe(ch2)

	hub.Broadcast(model.PowerEvent{PowerProduced: 1234})

	for _, ch := range []chan model.PowerEvent{ch1, ch2} {
		select {
		case got := <-ch:
			assert.Equal(t, 1234, got.PowerProduced)
		case <-time.After(time.Second):
			t.Fatal("client did not receive event")
		}
	}
}

func TestHub_SlowClientDoesNotBlockOthers(t *testing.T) {
	hub := api.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	fast := hub.Subscribe()
	slow := hub.Subscribe() // never read from
	defer hub.Unsubscribe(fast)
	defer hub.Unsubscribe(slow)

	for i := 0; i < 20; i++ {
		hub.Broadcast(model.PowerEvent{PowerProduced: i})
	}

	// Give the hub goroutine time to drain its broadcast queue and deliver to clients
	// before we attempt a non-blocking drain of the fast channel.
	time.Sleep(50 * time.Millisecond)

	received := 0
	for {
		select {
		case <-fast:
			received++
		default:
			goto done
		}
	}
done:
	assert.Greater(t, received, 0)
}

func TestHub_UnsubscribedClientStopsReceiving(t *testing.T) {
	hub := api.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	ch := hub.Subscribe()
	hub.Unsubscribe(ch)
	time.Sleep(20 * time.Millisecond)

	hub.Broadcast(model.PowerEvent{PowerProduced: 999})
	time.Sleep(20 * time.Millisecond)

	select {
	case _, open := <-ch:
		assert.False(t, open, "channel should be closed after unsubscribe")
	default:
	}
}

// TestServeSSE_EndToEnd verifies the full HTTP contract:
// Content-Type header, data: framing, and that an event is delivered.
func TestServeSSE_EndToEnd(t *testing.T) {
	hub := api.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	srv := httptest.NewServer(http.HandlerFunc(hub.ServeSSE))
	defer srv.Close()

	// Connect a client in a goroutine
	received := make(chan string, 1)
	go func() {
		resp, err := http.Get(srv.URL)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data:") {
				received <- line
				return
			}
		}
	}()

	time.Sleep(50 * time.Millisecond) // let the client connect

	hub.Broadcast(model.PowerEvent{PowerProduced: 7777})

	select {
	case line := <-received:
		assert.Contains(t, line, "7777")
	case <-time.After(2 * time.Second):
		t.Fatal("SSE client did not receive event within 2s")
	}
}
