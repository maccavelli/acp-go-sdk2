package clienthost

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestOutputTruncatesAtCharacter requires the output kept past its limit to
// be the stream's end, valid UTF-8, beginning at a character boundary, at
// most the limit and at most utf8.UTFMax-1 bytes short of it, and truncated,
// for characters of 1 to 4 bytes and a limit at each offset into one; both
// on a read, and once the output has been cut back
// (0041-PLAN-close-the-gaps-the-consumers-wait-on.md, phase 8).
func TestOutputTruncatesAtCharacter(t *testing.T) {
	t.Parallel()
	for _, char := range []string{"a", "é", "€", "😀"} {
		stream := strings.Repeat(char, 1000) + "z"
		for limit := 100; limit < 100+len(char); limit++ {
			for _, compact := range []bool{false, true} {
				o := &output{limit: limit}
				_, _ = o.Write([]byte(stream))
				if compact {
					_, _ = o.Write(bytes.Repeat([]byte(char), (limit+compactAfter)/len(char)+1))
					_, _ = o.Write([]byte("z"))
				}
				got, truncated := o.snapshot(true)
				all := string(o.buf)
				switch {
				case !utf8.ValidString(got):
					t.Errorf("%q, limit %d, compacted %v: the output is not valid UTF-8: %q…", char, limit, compact, got[:min(8, len(got))])
				case !strings.HasSuffix(all, got) || !strings.HasSuffix(got, "z"):
					t.Errorf("%q, limit %d, compacted %v: the output is not the stream's end", char, limit, compact)
				case len(got) > limit || len(got) < limit-(utf8.UTFMax-1):
					t.Errorf("%q, limit %d, compacted %v: kept %d bytes, want at most %d and at least %d", char, limit, compact, len(got),
						limit, limit-(utf8.UTFMax-1))
				case !truncated:
					t.Errorf("%q, limit %d, compacted %v: truncated = false", char, limit, compact)
				}
			}
		}
	}
}

// TestOutputHoldsBackUnfinishedCharacter requires a read of a running
// command's output to leave out a character the command has not finished
// writing, and a final read to keep it; and output within its limit to be
// kept whole, not truncated.
func TestOutputHoldsBackUnfinishedCharacter(t *testing.T) {
	t.Parallel()
	for _, char := range []string{"é", "€", "😀"} {
		for split := 1; split < len(char); split++ {
			o := &output{limit: 1 << 10}
			_, _ = o.Write([]byte("ab" + char[:split]))
			if got, truncated := o.snapshot(false); got != "ab" || truncated {
				t.Errorf("%q split at %d, running: %q, %v; want %q, false", char, split, got, truncated, "ab")
			}
			if got, _ := o.snapshot(true); got != "ab"+char[:split] {
				t.Errorf("%q split at %d, final: %q, want every byte written", char, split, got)
			}
			_, _ = o.Write([]byte(char[split:]))
			if got, _ := o.snapshot(false); got != "ab"+char {
				t.Errorf("%q, completed: %q, want %q", char, got, "ab"+char)
			}
		}
	}
	o := &output{limit: 3}
	_, _ = o.Write([]byte("abc"))
	if got, truncated := o.snapshot(false); got != "abc" || truncated {
		t.Errorf("output at its limit: %q, %v; want %q, false", got, truncated, "abc")
	}
}
