package main

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// TestServeAndDial runs serve on loopback and dial against it over each
// profile: the echoed prompt, end_turn, and a load on a second connection
// that replays the one update (0043-PLAN-remote-http-and-websocket-transport.md,
// phase 7).
func TestServeAndDial(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var served bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- serve(ctx, ln, &served) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("serve = %v, want nil after its context ends", err)
			}
		case <-time.After(2 * shutdownBound):
			t.Errorf("serve did not return within %v of its context's end", 2*shutdownBound)
		}
	})

	for _, scheme := range []string{"http", "ws"} {
		t.Run(scheme, func(t *testing.T) {
			var out bytes.Buffer
			if err := run(t.Context(), []string{"dial", scheme + "://" + ln.Addr().String() + "/acp"}, &out); err != nil {
				t.Fatalf("dial = %v, output %q", err, out.String())
			}
			if want := "echo: hello\nstop reason: end_turn\nloaded: 1 updates\n"; out.String() != want {
				t.Errorf("dial printed %q, want %q", out.String(), want)
			}
		})
	}
}

// TestRunUsage requires a usage error for no mode, a dial with no URL and an
// unknown mode.
func TestRunUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"dial"}, {"listen"}} {
		if err := run(t.Context(), args, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "usage") && !strings.Contains(err.Error(), "unknown mode") {
			t.Errorf("run(%q) = %v, want a usage error", args, err)
		}
	}
}
