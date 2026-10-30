package remote

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

// TestLineSplitter requires each complete line to be sent once, joined across
// writes, an unended line to be dropped at Close, and an over-long line to be
// dropped with errLineTooLong while the lines around it are sent
// (0043-PLAN-remote-http-and-websocket-transport.md, phase 3).
func TestLineSplitter(t *testing.T) {
	t.Parallel()
	var got []string
	s := newLineSplitter(func(line []byte) error {
		got = append(got, string(line))
		return nil
	}, 8)
	for _, w := range []string{`{"a"`, `:1}` + "\n", "x\ny\n", "held"} {
		if _, err := s.Write([]byte(w)); err != nil {
			t.Fatalf("Write(%q) = %v", w, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if want := []string{`{"a":1}`, "x", "y"}; !slices.Equal(got, want) {
		t.Errorf("lines sent = %q, want %q (joined across writes, the unended line dropped)", got, want)
	}

	got = nil
	n, err := s.Write([]byte("ok\n123456789\nafter\n"))
	if !errors.Is(err, errLineTooLong) || n != len("ok\n123456789\nafter\n") {
		t.Errorf("Write with a 9-byte line over an 8-byte limit = %d, %v, want all bytes and errLineTooLong", n, err)
	}
	if want := []string{"ok", "after"}; !slices.Equal(got, want) {
		t.Errorf("lines sent = %q, want %q", got, want)
	}

	got = nil
	if _, err := s.Write([]byte("1234")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write([]byte("56789")); !errors.Is(err, errLineTooLong) {
		t.Errorf("a line growing past the limit across writes: err = %v, want errLineTooLong", err)
	}
	if _, err := s.Write([]byte("tail\nnext\n")); err != nil {
		t.Errorf("the rest of the over-long line, then a line: err = %v, want nil", err)
	}
	if want := []string{"next"}; !slices.Equal(got, want) {
		t.Errorf("lines sent = %q, want %q (the over-long line's tail dropped)", got, want)
	}

	boom := errors.New("boom")
	failing := newLineSplitter(func([]byte) error { return boom }, 8)
	if n, err := failing.Write([]byte("a\nb\n")); !errors.Is(err, boom) || n != 0 {
		t.Errorf("Write with a failing send = %d, %v, want 0, boom", n, err)
	}
}

// TestToLine requires a message to become one line: JSON compacted, with the
// escapes of its strings kept, and anything else with its CR and LF made
// spaces (0043-MADR-remote-http-and-websocket-transport.md, "Amended by the
// PLAN, 2026-10-29", 6).
func TestToLine(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ in, want string }{
		{"{\n  \"id\": 1,\n  \"method\": \"x\"\n}", `{"id":1,"method":"x"}` + "\n"},
		{`{"text":"a\nb"}`, `{"text":"a\nb"}` + "\n"},
		{"not\r\njson\n", "not  json \n"},
		{"", "\n"},
	} {
		if got := string(toLine([]byte(tc.in))); got != tc.want {
			t.Errorf("toLine(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestLineQueue requires lines to be read in order across reads of any size,
// end's error after every line pushed before it, a push after end refused,
// and a push past queueLimit to wait for Read.
func TestLineQueue(t *testing.T) {
	t.Parallel()
	q := newLineQueue()
	q.push([]byte("one\n"))
	q.push([]byte("two\n"))
	q.end(nil)
	if q.push([]byte("late\n")) {
		t.Error("push after end = true, want false")
	}
	b, err := io.ReadAll(iotest.OneByteReader(q))
	if err != nil || string(b) != "one\ntwo\n" {
		t.Errorf("read = %q, %v, want the two lines and io.EOF", b, err)
	}

	q = newLineQueue()
	for range queueLimit {
		q.push([]byte("x\n"))
	}
	pushed := make(chan bool)
	go func() { pushed <- q.push([]byte("over\n")) }()
	select {
	case <-pushed:
		t.Fatal("a push past queueLimit returned before any read")
	case <-time.After(50 * time.Millisecond):
	}
	if _, err := q.Read(make([]byte, 2)); err != nil {
		t.Fatal(err)
	}
	select {
	case ok := <-pushed:
		if !ok {
			t.Error("the waiting push = false, want true")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the waiting push did not return within 5s of a read")
	}
	stop := errors.New("stop")
	q.end(stop)
	b, err = io.ReadAll(q)
	if !errors.Is(err, stop) || strings.Count(string(b), "\n") != queueLimit {
		t.Errorf("after end: %d lines, %v; want the %d left and stop", strings.Count(string(b), "\n"), err, queueLimit)
	}
}
