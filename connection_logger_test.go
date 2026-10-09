package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// lockedBuffer is a bytes.Buffer safe for the connection's goroutines and the
// test.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func noopHandler(context.Context, string, json.RawMessage) (any, *RequestError) { return nil, nil }

// A logger given with WithLogger is in place before the connection reads, so
// the diagnostic of an input that ends at once reaches it.
func TestWithLoggerReceivesEarlyDiagnostics(t *testing.T) {
	var buf lockedBuffer
	r, w := io.Pipe()
	_ = w.Close()
	c := NewConnection(noopHandler, io.Discard, r, WithLogger(slog.New(slog.NewTextHandler(&buf, nil))))
	<-c.Done()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(buf.String(), "connection closed") {
		if time.Now().After(deadline) {
			t.Fatalf("the WithLogger logger did not receive the close diagnostic; got %q", buf.String())
		}
		time.Sleep(time.Millisecond)
	}
}

// SetLogger while the connection's goroutines run is not a data race. Run
// with -race: the input ends at once, so the reader logs its close while
// SetLogger stores.
func TestSetLoggerWhileReading(t *testing.T) {
	for range 50 {
		r, w := io.Pipe()
		c := NewConnection(noopHandler, io.Discard, r)
		_ = w.Close()
		c.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
		<-c.Done()
	}
}

// A nil WithLogger keeps the default logger.
func TestWithLoggerNil(t *testing.T) {
	r, w := io.Pipe()
	defer w.Close()
	c := NewConnection(noopHandler, io.Discard, r, WithLogger(nil))
	if c.loggerOrDefault() != slog.Default() {
		t.Fatal("WithLogger(nil) replaced the default logger")
	}
}
