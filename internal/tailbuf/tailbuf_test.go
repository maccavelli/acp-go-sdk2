package tailbuf

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTruncatesAtCharacter requires the bytes kept past the limit to be the
// stream's end, valid UTF-8, beginning at a character boundary, at most the
// limit and at most utf8.UTFMax-1 bytes short of it, and some dropped, for
// characters of 1 to 4 bytes and a limit at each offset into one; both on a
// read, and once the buffer has been cut back
// (0041-PLAN-close-the-gaps-the-consumers-wait-on.md, phase 8, where it was
// clienthost's TestOutputTruncatesAtCharacter).
func TestTruncatesAtCharacter(t *testing.T) {
	t.Parallel()
	for _, char := range []string{"a", "é", "€", "😀"} {
		stream := strings.Repeat(char, 1000) + "z"
		for limit := 100; limit < 100+len(char); limit++ {
			for _, compact := range []bool{false, true} {
				b := &Buffer{Limit: limit}
				_, _ = b.Write([]byte(stream))
				if compact {
					_, _ = b.Write(bytes.Repeat([]byte(char), (limit+compactAfter)/len(char)+1))
					_, _ = b.WriteString("z")
				}
				got, dropped := b.Snapshot(true)
				all := string(b.buf)
				switch {
				case !utf8.ValidString(got):
					t.Errorf("%q, limit %d, compacted %v: the text is not valid UTF-8: %q…", char, limit, compact, got[:min(8, len(got))])
				case !strings.HasSuffix(all, got) || !strings.HasSuffix(got, "z"):
					t.Errorf("%q, limit %d, compacted %v: the text is not the stream's end", char, limit, compact)
				case len(got) > limit || len(got) < limit-(utf8.UTFMax-1):
					t.Errorf("%q, limit %d, compacted %v: kept %d bytes, want at most %d and at least %d", char, limit, compact, len(got),
						limit, limit-(utf8.UTFMax-1))
				case dropped == 0:
					t.Errorf("%q, limit %d, compacted %v: nothing dropped", char, limit, compact)
				}
			}
		}
	}
}

// TestHoldsBackUnfinishedCharacter requires a read that is not final to leave
// out a character the stream has not finished writing, and a final read to
// keep it; and a stream within its limit to be kept whole, nothing dropped
// (0041-PLAN-close-the-gaps-the-consumers-wait-on.md, phase 8, where it was
// clienthost's TestOutputHoldsBackUnfinishedCharacter).
func TestHoldsBackUnfinishedCharacter(t *testing.T) {
	t.Parallel()
	for _, char := range []string{"é", "€", "😀"} {
		for split := 1; split < len(char); split++ {
			b := &Buffer{Limit: 1 << 10}
			_, _ = b.Write([]byte("ab" + char[:split]))
			if got, dropped := b.Snapshot(false); got != "ab" || dropped != 0 {
				t.Errorf("%q split at %d, not final: %q, %d dropped; want %q, 0", char, split, got, dropped, "ab")
			}
			if got, _ := b.Snapshot(true); got != "ab"+char[:split] {
				t.Errorf("%q split at %d, final: %q, want every byte written", char, split, got)
			}
			_, _ = b.Write([]byte(char[split:]))
			if got, _ := b.Snapshot(false); got != "ab"+char {
				t.Errorf("%q, completed: %q, want %q", char, got, "ab"+char)
			}
		}
	}
	b := &Buffer{Limit: 3}
	_, _ = b.Write([]byte("abc"))
	if got, dropped := b.Snapshot(false); got != "abc" || dropped != 0 {
		t.Errorf("a stream at its limit: %q, %d dropped; want %q, 0", got, dropped, "abc")
	}
}

// TestDroppedCount requires Snapshot's count to be every byte written less
// the bytes it returns, compactions included, for a final read; and a read
// that holds back a character not to count it as dropped
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D4).
func TestDroppedCount(t *testing.T) {
	t.Parallel()
	const limit = 1000
	b := &Buffer{Limit: limit}
	written := 0
	for range 200 {
		n, _ := b.WriteString(strings.Repeat("€", 1000))
		written += n
	}
	got, dropped := b.Snapshot(true)
	if want := int64(written - len(got)); dropped != want {
		t.Errorf("dropped %d, want %d: %d written, %d kept", dropped, want, written, len(got))
	}
	if d := b.Dropped(); d != dropped {
		t.Errorf("Dropped() = %d, want Snapshot's %d", d, dropped)
	}
	_, _ = b.Write([]byte("€")[:2])
	held, heldDropped := b.Snapshot(false)
	final, finalDropped := b.Snapshot(true)
	if heldDropped > finalDropped || len(held) >= len(final) {
		t.Errorf("not final: %d bytes, %d dropped; final: %d bytes, %d dropped; want the held-back bytes not counted", len(held),
			heldDropped, len(final), finalDropped)
	}
}

// TestWriteNeverFails requires Write and WriteString to accept every byte,
// as io.Writer's contract asks of a writer that returns no error.
func TestWriteNeverFails(t *testing.T) {
	t.Parallel()
	b := &Buffer{Limit: 4}
	p := bytes.Repeat([]byte("x"), 3*compactAfter)
	if n, err := b.Write(p); n != len(p) || err != nil {
		t.Errorf("Write = %d, %v; want %d, nil", n, err, len(p))
	}
	if n, err := b.WriteString("yz"); n != 2 || err != nil {
		t.Errorf("WriteString = %d, %v; want 2, nil", n, err)
	}
}
