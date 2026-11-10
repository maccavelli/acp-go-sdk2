// Package tailbuf keeps the end of a byte stream within a limit, dropping
// from the front at a character boundary, as the ACP schema's
// outputByteLimit requires of a terminal's output: "The Client MUST ensure
// truncation happens at a character boundary to maintain valid string
// output, even if this means the retained output is slightly less than the
// specified limit." clienthost's terminals and acp's ToolCallStream share it
// (0041-PLAN-close-the-gaps-the-consumers-wait-on.md, phase 8;
// 0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D4).
package tailbuf

import (
	"sync"
	"unicode/utf8"
)

// compactAfter is how far past its limit a Buffer may grow before it is cut
// back: a cut moves what is kept, so cutting on every write past the limit
// would move the limit's bytes per write.
const compactAfter = 64 << 10

// Buffer is the end of a stream, at most Limit bytes of it when read. Set
// Limit, which must be positive, before the first Write. It is safe for
// concurrent use.
type Buffer struct {
	// Limit is the most bytes Snapshot returns.
	Limit int

	mu      sync.Mutex
	buf     []byte
	dropped int64 // bytes cut from the front by a compaction
}

// Append keeps p, and cuts the buffer back to its limit once it has grown
// compactAfter past it.
func (b *Buffer) Append(p []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	b.compact()
}

// AppendString is Append for a string, without converting it to a []byte.
func (b *Buffer) AppendString(s string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, s...)
	b.compact()
}

// Write is Append as an io.Writer: it never fails, and returns len(p).
func (b *Buffer) Write(p []byte) (int, error) {
	b.Append(p)
	return len(p), nil
}

// WriteString is AppendString as an io.StringWriter: it never fails, and
// returns len(s).
func (b *Buffer) WriteString(s string) (int, error) {
	b.AppendString(s)
	return len(s), nil
}

// compact moves the last Limit bytes, from a character boundary, to the
// front, once the buffer is compactAfter past its limit. The caller holds mu.
func (b *Buffer) compact() {
	if len(b.buf) <= b.Limit+compactAfter {
		return
	}
	i := cut(b.buf, b.Limit)
	b.dropped += int64(i)
	b.buf = b.buf[:copy(b.buf, b.buf[i:])]
}

// Snapshot returns the stream's end, at most Limit bytes, beginning at a
// character boundary, and how many bytes have been dropped from its front so
// far. Unless final, it holds back a character the stream's last write began
// and did not finish, which a later write completes; held back is not
// dropped.
func (b *Buffer) Snapshot(final bool) (text string, dropped int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	kept, dropped := b.buf, b.dropped
	if len(kept) > b.Limit {
		i := cut(kept, b.Limit)
		kept = kept[i:]
		dropped += int64(i)
	}
	if !final {
		kept = kept[:complete(kept)]
	}
	return string(kept), dropped
}

// Dropped returns how many bytes have been dropped from the stream's front so
// far, as Snapshot counts them, without copying what is kept.
func (b *Buffer) Dropped() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.buf) > b.Limit {
		return b.dropped + int64(cut(b.buf, b.Limit))
	}
	return b.dropped
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
