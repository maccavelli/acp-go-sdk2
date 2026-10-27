package clienthost

import (
	"sync"
	"unicode/utf8"
)

// compactAfter is how far past its limit a terminal's output may grow before
// it is cut back: a cut moves what is kept, so cutting on every write past
// the limit would copy the limit's bytes per write.
const compactAfter = 64 << 10

// output is a terminal's output, kept to its limit by dropping from the
// front at a character boundary, as the schema's outputByteLimit requires:
// "The Client MUST ensure truncation happens at a character boundary to
// maintain valid string output, even if this means the retained output is
// slightly less than the specified limit."
type output struct {
	mu        sync.Mutex
	limit     int
	buf       []byte
	truncated bool // a byte has been dropped
}

// Write keeps p, and cuts the output back to its limit once it has grown
// compactAfter past it. It never fails.
func (o *output) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.buf = append(o.buf, p...)
	if len(o.buf) > o.limit+compactAfter {
		o.buf = append([]byte(nil), o.buf[cut(o.buf, o.limit):]...)
		o.truncated = true
	}
	return len(p), nil
}

// snapshot returns the output kept, at most the limit's bytes, beginning at
// a character boundary, and whether any was dropped. Unless final, it holds
// back a character the command has not finished writing, which a later
// write completes.
func (o *output) snapshot(final bool) (string, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	b, truncated := o.buf, o.truncated
	if len(b) > o.limit {
		b = b[cut(b, o.limit):]
		truncated = true
	}
	if !final {
		b = b[:complete(b)]
	}
	return string(b), truncated
}

// cut returns where b's last limit bytes begin, moved on past the
// continuation bytes of a character the limit splits.
func cut(b []byte, limit int) int {
	i := max(len(b)-limit, 0)
	for n := 0; i < len(b) && n < utf8.UTFMax-1 && !utf8.RuneStart(b[i]); n++ {
		i++
	}
	return i
}

// complete returns the length of b without a character its last bytes
// begin and do not finish.
func complete(b []byte) int {
	for i := len(b) - 1; i >= 0 && i >= len(b)-utf8.UTFMax; i-- {
		if utf8.RuneStart(b[i]) {
			if utf8.FullRune(b[i:]) {
				return len(b)
			}
			return i
		}
	}
	return len(b)
}
