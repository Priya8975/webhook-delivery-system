package websocket

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func setupTestHub(t *testing.T) *Hub {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	hub := NewHub(logger)
	go hub.Run()
	return hub
}

func connectWS(t *testing.T, hub *Hub) (*websocket.Conn, func()) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect WebSocket: %v", err)
	}

	cleanup := func() {
		conn.Close()
		server.Close()
	}

	return conn, cleanup
}

func TestHub_ClientConnects(t *testing.T) {
	hub := setupTestHub(t)

	conn, cleanup := connectWS(t, hub)
	defer cleanup()

	// Give the hub time to register the client
	time.Sleep(50 * time.Millisecond)

	if count := hub.ClientCount(); count != 1 {
		t.Errorf("expected 1 client, got %d", count)
	}

	conn.Close()
	time.Sleep(50 * time.Millisecond)

	if count := hub.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", count)
	}
}

func TestHub_BroadcastReachesClient(t *testing.T) {
	hub := setupTestHub(t)

	conn, cleanup := connectWS(t, hub)
	defer cleanup()

	time.Sleep(50 * time.Millisecond)

	// Broadcast an event
	hub.Broadcast(DeliveryEvent{
		Type:         "delivery_success",
		EventID:      "evt-123",
		SubscriberID: "sub-456",
		EndpointURL:  "http://example.com/webhook",
		EventType:    "order.created",
		Attempt:      1,
		ResponseMs:   42,
		Timestamp:    time.Now(),
	})

	// Read the message from the WebSocket
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	msg := string(message)
	if !strings.Contains(msg, "delivery_success") {
		t.Errorf("expected message to contain 'delivery_success', got: %s", msg)
	}
	if !strings.Contains(msg, "evt-123") {
		t.Errorf("expected message to contain event ID, got: %s", msg)
	}
}

func TestHub_MultipleClients(t *testing.T) {
	hub := setupTestHub(t)

	conn1, cleanup1 := connectWS(t, hub)
	defer cleanup1()
	conn2, cleanup2 := connectWS(t, hub)
	defer cleanup2()

	time.Sleep(50 * time.Millisecond)

	if count := hub.ClientCount(); count != 2 {
		t.Errorf("expected 2 clients, got %d", count)
	}

	// Broadcast should reach both clients
	hub.Broadcast(DeliveryEvent{
		Type:    "delivery_success",
		EventID: "evt-multi",
	})

	for i, conn := range []*websocket.Conn{conn1, conn2} {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("client %d failed to read: %v", i+1, err)
		}
		if !strings.Contains(string(message), "evt-multi") {
			t.Errorf("client %d didn't receive broadcast", i+1)
		}
	}
}

func TestHub_ClientCountStartsAtZero(t *testing.T) {
	hub := setupTestHub(t)

	if count := hub.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients initially, got %d", count)
	}
}
