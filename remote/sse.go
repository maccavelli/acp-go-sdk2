package remote

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

// sseReader reads the events of a text/event-stream body, the format
// Streamable HTTP's GET streams carry (https://html.spec.whatwg.org/multipage/server-sent-events.html):
// lines end in LF, CR or CRLF; an event's data lines are joined with "\n";
// a blank line ends an event; a comment line, ":…", and the event, id and
// retry fields are ignored, since the RFD's v1 uses none of them.
type sseReader struct {
	r        *bufio.Reader
	maxBytes int
}

// newSSEReader returns a reader of r's events, each at most maxBytes of data.
func newSSEReader(r io.Reader, maxBytes int) *sseReader {
	return &sseReader{r: bufio.NewReader(r), maxBytes: maxBytes}
}

// next returns the next event's data. It returns io.EOF at the stream's end,
// an event not ended by a blank line being no event, and errLineTooLong for an
// event or a line over maxBytes, which ends the stream.
func (s *sseReader) next() ([]byte, error) {
	var data []byte
	var has bool
	for {
		line, err := s.readLine()
		if err != nil {
			return nil, err
		}
		if len(line) == 0 {
			if has {
				return data, nil
			}
			continue
		}
		if line[0] == ':' {
			continue
		}
		field, value, _ := bytes.Cut(line, []byte(":"))
		value = bytes.TrimPrefix(value, []byte(" "))
		if string(field) != "data" {
			continue
		}
		if has {
			data = append(data, '\n')
		}
		data = append(data, value...)
		has = true
		if len(data) > s.maxBytes {
			return nil, fmt.Errorf("%w: an event of more than %d bytes", errLineTooLong, s.maxBytes)
		}
	}
}

// readLine reads one line without its ending, LF, CR or CRLF. A line longer
// than maxBytes, with room for its field name, is errLineTooLong.
func (s *sseReader) readLine() ([]byte, error) {
	var line []byte
	for {
		b, err := s.r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) && len(line) > 0 {
				// A last line with no ending is not a line: it ends no event.
				return nil, io.EOF
			}
			return nil, err
		}
		switch b {
		case '\n':
			return line, nil
		case '\r':
			if next, err := s.r.Peek(1); err == nil && next[0] == '\n' {
				_, _ = s.r.ReadByte() //nolint:errcheck // Peek has just seen it.
			}
			return line, nil
		}
		line = append(line, b)
		if len(line) > s.maxBytes+len("data: ") {
			return nil, fmt.Errorf("%w: a line of more than %d bytes", errLineTooLong, s.maxBytes)
		}
	}
}
