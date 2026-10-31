package acp

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"
)

func TestWithMaxQueuedNotifications_DefaultSize(t *testing.T) {
	// Verify default queue capacity without options.
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	t.Cleanup(func() {
		_ = c2aR.Close()
		_ = c2aW.Close()
		_ = a2cR.Close()
		_ = a2cW.Close()
	})

	client := &clientFuncs{}
	csc := NewClientSideConnection(client, c2aW, a2cR)

	if csc.conn.queueLimit != defaultMaxQueuedNotifications {
		t.Fatalf("default queue capacity = %d, want %d", csc.conn.queueLimit, defaultMaxQueuedNotifications)
	}
	// Under OverflowBlock, the default, the channel holds twice the limit, for
	// the reader's read-on (0044-MADR-notification-queue-back-pressure.md, D4).
	if got := cap(csc.conn.notificationQueue); got != 2*defaultMaxQueuedNotifications {
		t.Fatalf("default queue channel capacity = %d, want %d", got, 2*defaultMaxQueuedNotifications)
	}
}

func TestWithMaxQueuedNotifications_CustomSize(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	t.Cleanup(func() {
		_ = c2aR.Close()
		_ = c2aW.Close()
		_ = a2cR.Close()
		_ = a2cW.Close()
	})

	const customSize = 8192
	client := &clientFuncs{}
	csc := NewClientSideConnection(client, c2aW, a2cR, WithMaxQueuedNotifications(customSize))

	if csc.conn.queueLimit != customSize {
		t.Fatalf("custom queue capacity = %d, want %d", csc.conn.queueLimit, customSize)
	}
}

func TestWithMaxQueuedNotifications_AgentSide(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	t.Cleanup(func() {
		_ = c2aR.Close()
		_ = c2aW.Close()
		_ = a2cR.Close()
		_ = a2cW.Close()
	})

	const customSize = 4096
	asc := NewAgentSideConnection(agentFuncs{}, a2cW, c2aR, WithMaxQueuedNotifications(customSize))

	if asc.conn.queueLimit != customSize {
		t.Fatalf("agent queue capacity = %d, want %d", asc.conn.queueLimit, customSize)
	}
}

func TestWithMaxQueuedNotifications_ZeroIgnored(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	t.Cleanup(func() {
		_ = c2aR.Close()
		_ = c2aW.Close()
		_ = a2cR.Close()
		_ = a2cW.Close()
	})

	client := &clientFuncs{}
	csc := NewClientSideConnection(client, c2aW, a2cR, WithMaxQueuedNotifications(0))

	if csc.conn.queueLimit != defaultMaxQueuedNotifications {
		t.Fatalf("zero option should keep default: got %d, want %d", csc.conn.queueLimit, defaultMaxQueuedNotifications)
	}
}

func TestWithMaxQueuedNotifications_NegativeIgnored(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	t.Cleanup(func() {
		_ = c2aR.Close()
		_ = c2aW.Close()
		_ = a2cR.Close()
		_ = a2cW.Close()
	})

	client := &clientFuncs{}
	csc := NewClientSideConnection(client, c2aW, a2cR, WithMaxQueuedNotifications(-5))

	if csc.conn.queueLimit != defaultMaxQueuedNotifications {
		t.Fatalf("negative option should keep default: got %d, want %d", csc.conn.queueLimit, defaultMaxQueuedNotifications)
	}
}

func TestLargerQueue_SurvivesNotificationBurst(t *testing.T) {
	// With a tiny queue (2), a burst of notifications should overflow.
	// With a larger queue, it should survive.
	const burstSize = 50

	var mu sync.Mutex
	var received int

	client := &clientFuncs{
		SessionUpdateFunc: func(_ context.Context, n SessionNotification) error {
			// Simulate slow processing
			time.Sleep(time.Millisecond)
			mu.Lock()
			received++
			mu.Unlock()
			return nil
		},
	}

	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	t.Cleanup(func() {
		_ = c2aR.Close()
		_ = c2aW.Close()
		_ = a2cR.Close()
		_ = a2cW.Close()
	})

	// Use a queue large enough to absorb the burst.
	clientConn := NewClientSideConnection(client, c2aW, a2cR, WithMaxQueuedNotifications(burstSize*2))
	agentConn := NewAgentSideConnection(agentFuncs{}, a2cW, c2aR, WithMaxQueuedNotifications(burstSize*2))
	negotiate(t, clientConn, allAdvertised())
	negotiate(t, agentConn, allAdvertised())

	for i := range burstSize {
		if err := agentConn.SessionUpdate(t.Context(), testSessionUpdate("burst", i)); err != nil {
			t.Fatalf("SessionUpdate %d failed: %v", i, err)
		}
	}

	// Wait for all notifications to be processed.
	waitForNotificationBarrierDrain(t, clientConn.conn, 5*time.Second)

	mu.Lock()
	got := received
	mu.Unlock()

	if got != burstSize {
		t.Fatalf("received %d notifications, want %d", got, burstSize)
	}
}
