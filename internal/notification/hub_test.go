package notification

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHub_ConnectedCount(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	if hub.ConnectedCount() != 0 {
		t.Errorf("initial count = %d, want 0", hub.ConnectedCount())
	}
}

func TestHub_RegisterAndUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	userID := uuid.New()
	client := &Client{
		UserID: userID,
		Send:   make(chan []byte, 256),
	}

	// Register
	hub.register <- client
	time.Sleep(50 * time.Millisecond)

	if hub.ConnectedCount() != 1 {
		t.Errorf("after register, count = %d, want 1", hub.ConnectedCount())
	}

	// Unregister
	hub.unregister <- client
	time.Sleep(50 * time.Millisecond)

	if hub.ConnectedCount() != 0 {
		t.Errorf("after unregister, count = %d, want 0", hub.ConnectedCount())
	}
}

func TestHub_SendToUser(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	userID := uuid.New()
	client := &Client{
		UserID: userID,
		Send:   make(chan []byte, 256),
	}

	hub.register <- client
	time.Sleep(50 * time.Millisecond)

	// Send message
	hub.SendToUser(userID, "ride.matched", map[string]string{"ride_id": "test-123"})

	// Verify message received
	select {
	case msg := <-client.Send:
		if len(msg) == 0 {
			t.Error("received empty message")
		}
	case <-time.After(time.Second):
		t.Error("timed out waiting for message")
	}

	hub.unregister <- client
}

func TestHub_SendToUser_NonExistent(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Should not panic when sending to non-existent user
	hub.SendToUser(uuid.New(), "test.event", nil)
	time.Sleep(50 * time.Millisecond)
}

func TestHub_ReplacesOldConnection(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	userID := uuid.New()

	client1 := &Client{
		UserID: userID,
		Send:   make(chan []byte, 256),
	}
	client2 := &Client{
		UserID: userID,
		Send:   make(chan []byte, 256),
	}

	// Register first client
	hub.register <- client1
	time.Sleep(50 * time.Millisecond)

	// Register second client with same user ID
	hub.register <- client2
	time.Sleep(50 * time.Millisecond)

	// Should still be 1 connection (old one replaced)
	if hub.ConnectedCount() != 1 {
		t.Errorf("count = %d, want 1 (old connection should be replaced)", hub.ConnectedCount())
	}

	hub.unregister <- client2
}

func TestHub_ConcurrentRegistrations(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	var wg sync.WaitGroup
	numClients := 50

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &Client{
				UserID: uuid.New(),
				Send:   make(chan []byte, 256),
			}
			hub.register <- client
		}()
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	count := hub.ConnectedCount()
	if count != numClients {
		t.Errorf("count = %d, want %d", count, numClients)
	}
}

func TestHub_BroadcastToUsers(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	user1 := uuid.New()
	user2 := uuid.New()
	user3 := uuid.New()

	c1 := &Client{UserID: user1, Send: make(chan []byte, 256)}
	c2 := &Client{UserID: user2, Send: make(chan []byte, 256)}
	c3 := &Client{UserID: user3, Send: make(chan []byte, 256)}

	hub.register <- c1
	hub.register <- c2
	hub.register <- c3
	time.Sleep(50 * time.Millisecond)

	// Broadcast to user1 and user2 only
	hub.BroadcastToUsers([]uuid.UUID{user1, user2}, "test.event", "hello")
	time.Sleep(100 * time.Millisecond)

	// user1 and user2 should receive messages
	select {
	case <-c1.Send:
	case <-time.After(time.Second):
		t.Error("user1 didn't receive message")
	}

	select {
	case <-c2.Send:
	case <-time.After(time.Second):
		t.Error("user2 didn't receive message")
	}

	// user3 should NOT receive a message
	select {
	case <-c3.Send:
		t.Error("user3 should not have received message")
	case <-time.After(100 * time.Millisecond):
		// expected
	}

	hub.unregister <- c1
	hub.unregister <- c2
	hub.unregister <- c3
}
