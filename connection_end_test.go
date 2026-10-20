package acp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
)

// errOnRead is a reader whose every read fails with err.
type errOnRead struct{ err error }

func (r errOnRead) Read([]byte) (int, error) { return 0, r.err }

// ended waits for c's end and returns Err, read twice to require the same
// answer each time.
func ended(t *testing.T, c *Connection) error {
	t.Helper()
	within(t, c.Done(), "the connection's end")
	err := c.Err()
	if again := c.Err(); !errors.Is(again, err) {
		t.Errorf("Err returned %v, then %v", err, again)
	}
	return err
}

// TestErr requires Err to be nil until the connection ends and then to say
// why it ended, the same each time, with the cause each end gives readable by
// errors.Is: Close, the peer's end of output (io.EOF, which a later send's
// error wraps too), a read's error, and a notification overflow; and, after
// the peer's end, a later Close not to change it while a call then still
// returns ErrConnectionClosed
// (0026-MADR-connection-end-cause-and-wait.md, E1 to E6).
func TestErr(t *testing.T) {
	t.Parallel()
	is := func(t *testing.T, err error, want []error, not []error) {
		t.Helper()
		for _, w := range want {
			if !errors.Is(err, w) {
				t.Errorf("Err = %v, want it to wrap %v", err, w)
			}
		}
		for _, n := range not {
			if errors.Is(err, n) {
				t.Errorf("Err = %v, want it not to wrap %v", err, n)
			}
		}
	}
	t.Run("before the end", func(t *testing.T) {
		t.Parallel()
		c := NewConnection(nil, io.Discard, idleReader(t))
		if err := c.Err(); err != nil {
			t.Errorf("Err before the end = %v, want nil", err)
		}
		_ = c.Close()
	})
	t.Run("Close", func(t *testing.T) {
		t.Parallel()
		c := NewConnection(nil, io.Discard, idleReader(t))
		_ = c.Close()
		err := ended(t, c)
		if err == nil || err.Error() != "acp: connection closed" {
			t.Errorf("Err = %v, want acp: connection closed", err)
		}
		is(t, err, []error{ErrConnectionClosed}, []error{ErrPeerDisconnected, io.EOF})
	})
	t.Run("the peer's end of output", func(t *testing.T) {
		t.Parallel()
		pr, pw := io.Pipe()
		c := NewConnection(nil, io.Discard, pr)
		_ = pw.Close()
		err := ended(t, c)
		if err == nil || err.Error() != "acp: peer disconnected: peer connection closed" {
			t.Errorf("Err = %v, want acp: peer disconnected: peer connection closed", err)
		}
		is(t, err, []error{ErrPeerDisconnected, io.EOF}, []error{ErrConnectionClosed})
		send := c.SendNotification(context.Background(), "_x/n", map[string]any{})
		if !errors.Is(send, ErrPeerDisconnected) || !errors.Is(send, io.EOF) {
			t.Errorf("a send after the peer's end = %v, want ErrPeerDisconnected and io.EOF", send)
		}
	})
	t.Run("a read error", func(t *testing.T) {
		t.Parallel()
		reset := errors.New("read: connection reset")
		c := NewConnection(nil, io.Discard, errOnRead{reset})
		err := ended(t, c)
		is(t, err, []error{ErrPeerDisconnected, reset}, []error{ErrConnectionClosed, io.EOF})
	})
	t.Run("notification overflow", func(t *testing.T) {
		t.Parallel()
		pr, pw := io.Pipe()
		t.Cleanup(func() { _ = pw.Close() })
		h := func(ctx context.Context, _ string, _ json.RawMessage) (any, *RequestError) {
			<-ctx.Done()
			return nil, nil
		}
		c := NewConnection(h, io.Discard, pr, WithMaxQueuedNotifications(1))
		go func() {
			for range 4 {
				if _, err := pw.Write([]byte(`{"jsonrpc":"2.0","method":"_x/n","params":{}}` + "\n")); err != nil {
					return
				}
			}
		}()
		err := ended(t, c)
		if err == nil || err.Error() != "acp: connection closed: notification queue overflow" {
			t.Errorf("Err = %v, want acp: connection closed: notification queue overflow", err)
		}
		is(t, err, []error{ErrConnectionClosed, ErrNotificationQueueOverflow}, []error{ErrPeerDisconnected})
	})
	t.Run("the peer's end, then Close", func(t *testing.T) {
		t.Parallel()
		pr, pw := io.Pipe()
		c := NewConnection(nil, io.Discard, pr)
		_ = pw.Close()
		before := ended(t, c)
		_ = c.Close()
		if after := c.Err(); !errors.Is(after, before) {
			t.Errorf("Err after a late Close = %v, want the peer's end, %v", after, before)
		}
		is(t, c.Err(), []error{ErrPeerDisconnected, io.EOF}, []error{ErrConnectionClosed})
		if err := c.SendNotification(context.Background(), "_x/n", map[string]any{}); !errors.Is(err, ErrConnectionClosed) {
			t.Errorf("a send after the late Close = %v, want ErrConnectionClosed", err)
		}
	})
}
