package agentproc

import (
	"bytes"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// frames is a LineTap's fn that keeps each frame it is given.
type frames struct {
	mu  sync.Mutex
	got [][]byte
}

func (f *frames) add(frame []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, frame)
}

func (f *frames) strings() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := make([]string, len(f.got))
	for i, b := range f.got {
		s[i] = string(b)
	}
	return s
}

// lineStream is what TestLineTapFrames writes: lines of one byte and of none,
// a "\r\n" end, and a last line not ended.
const lineStream = "a\nbb\r\n\n{\"x\":1}\nccc"

// lineFrames is what a LineTap makes of lineStream.
var lineFrames = []string{"a", "bb\r", "", `{"x":1}`}

// TestLineTapFrames requires a LineTap to call fn with each line, without its
// newline and with a "\r" before it kept, however the stream is split into
// writes: whole, a byte at a time, and at every pair of points; and never
// with the line not ended (0041-PLAN-close-the-gaps-the-consumers-wait-on.md,
// phase 4).
func TestLineTapFrames(t *testing.T) {
	t.Parallel()
	run := func(t *testing.T, parts []string) {
		t.Helper()
		f := &frames{}
		w := LineTap(f.add)
		for _, p := range parts {
			if n, err := w.Write([]byte(p)); n != len(p) || err != nil {
				t.Fatalf("Write(%q) = %d, %v; want %d, nil", p, n, err, len(p))
			}
		}
		if got := f.strings(); !slices.Equal(got, lineFrames) {
			t.Errorf("writes %q gave frames %q, want %q", parts, got, lineFrames)
		}
	}
	t.Run("whole", func(t *testing.T) { run(t, []string{lineStream}) })
	t.Run("a byte at a time", func(t *testing.T) { run(t, strings.Split(lineStream, "")) })
	t.Run("every split", func(t *testing.T) {
		for i := range len(lineStream) + 1 {
			for j := i; j <= len(lineStream); j++ {
				run(t, []string{lineStream[:i], lineStream[i:j], lineStream[j:]})
			}
		}
	})
}

// TestLineTapCopies requires each frame to be a copy: fn may keep it and
// change it, and the writer's caller may reuse its buffer, without changing a
// frame already given or the next one.
func TestLineTapCopies(t *testing.T) {
	t.Parallel()
	// write writes "one\ntw" and then "o\nthree\n" from one buffer of its
	// own, spoiling the buffer after each write.
	write := func(w io.Writer) {
		buf := []byte("one\ntw")
		_, _ = w.Write(buf)
		copy(buf, "ZZZZZZ")
		buf = append(buf[:0], "o\nthree\n"...)
		_, _ = w.Write(buf)
		copy(buf, "ZZZZZZZZ")
	}
	f := &frames{}
	write(LineTap(func(frame []byte) {
		f.add(bytes.Clone(frame))
		for i := range frame {
			frame[i] = 'X'
		}
	}))
	kept := &frames{}
	write(LineTap(kept.add))
	want := []string{"one", "two", "three"}
	if got := f.strings(); !slices.Equal(got, want) {
		t.Errorf("frames after fn changed each = %q, want %q", got, want)
	}
	if got := kept.strings(); !slices.Equal(got, want) {
		t.Errorf("frames kept after the caller reused its buffer = %q, want %q", got, want)
	}
}

// TestLineTapHoldsUnended requires a line not ended to reach fn only with its
// newline, and never once Close has dropped it.
func TestLineTapHoldsUnended(t *testing.T) {
	t.Parallel()
	f := &frames{}
	w := LineTap(f.add)
	_, _ = w.Write([]byte("partial"))
	if got := f.strings(); len(got) != 0 {
		t.Fatalf("frames before a newline = %q, want none", got)
	}
	c, ok := w.(io.Closer)
	if !ok {
		t.Fatalf("LineTap's writer, %T, is not an io.Closer", w)
	}
	if err := c.Close(); err != nil {
		t.Errorf("Close = %v, want nil", err)
	}
	if got := f.strings(); len(got) != 0 {
		t.Errorf("frames after Close = %q, want none: what is not ended is not a frame", got)
	}
	_, _ = w.Write([]byte("next\n"))
	if got, want := f.strings(), []string{"next"}; !slices.Equal(got, want) {
		t.Errorf("frames after Close and a line = %q, want %q: Close dropped the held line", got, want)
	}
}

// TestLineTapNilFn requires LineTap to refuse a nil fn at once, not on the
// tap's goroutine.
func TestLineTapNilFn(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Error("LineTap(nil) did not panic")
		}
	}()
	LineTap(nil)
}

// TestLineTapAsOutputTap requires a LineTap set as Command.OutputTap to give
// the agent's lines, as it wrote them: the "frames" child's line too long for
// the connection's limit, its line that is not JSON, and its notification.
func TestLineTapAsOutputTap(t *testing.T) {
	t.Parallel()
	f := &frames{}
	p := startTapped(t, "frames", nil, testGrace, LineTap(f.add), nil, acp.WithMaxMessageSize(1<<20))
	if err := p.Close(); err != nil {
		t.Fatalf("Close = %v", err)
	}
	want := strings.Split(strings.TrimSuffix(string(framesStream()), "\n"), "\n")
	got := f.strings()
	if !slices.Equal(got, want) {
		t.Errorf("the output tap's frames: %d, want %d; equal %v", len(got), len(want), slices.Equal(got, want))
	}
}
