package acptest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ReadRecords reads records written one JSON object a line, as WriteRecords
// and agentproc.Transcript write them: {"direction":…,"frame":…}. A blank line
// is skipped. A line that is not such an object, or whose direction is
// neither Sent nor Received, is an error naming its line
// (0042-MADR-in-process-agent-runner.md, item 4).
func ReadRecords(r io.Reader) ([]Record, error) {
	br := bufio.NewReader(r)
	var records []Record
	for n := 1; ; n++ {
		line, err := br.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			var rec Record
			if uerr := json.Unmarshal(line, &rec); uerr != nil {
				return records, fmt.Errorf("acptest: records, line %d: %w", n, uerr)
			}
			if rec.Direction != Sent && rec.Direction != Received {
				return records, fmt.Errorf("acptest: records, line %d: direction %q is neither %q nor %q", n, rec.Direction, Sent, Received)
			}
			records = append(records, rec)
		}
		switch {
		case errors.Is(err, io.EOF):
			return records, nil
		case err != nil:
			return records, fmt.Errorf("acptest: records, line %d: %w", n, err)
		}
	}
}

// WriteRecords writes records one JSON object a line, as ReadRecords reads
// them. A frame that is not JSON is written as a JSON string, so that the
// line stays one object.
func WriteRecords(w io.Writer, records []Record) error {
	var b bytes.Buffer
	for _, r := range records {
		frame := r.Frame
		if !json.Valid(frame) {
			s, err := json.Marshal(string(frame))
			if err != nil {
				return err
			}
			frame = s
		}
		line, err := json.Marshal(Record{Direction: r.Direction, Frame: frame})
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	_, err := w.Write(b.Bytes())
	return err
}

// Decode returns the params of each record that went the way d with method,
// a request or a notification, decoded into T, in the records' order. A
// record whose params T refuses is an error naming the record; the others
// are skipped.
func Decode[T any](records []Record, d Direction, method string) ([]T, error) {
	var out []T
	for i, r := range records {
		if r.Direction != d {
			continue
		}
		var m struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(r.Frame, &m) != nil || m.Method != method {
			continue
		}
		if m.Params == nil {
			m.Params = json.RawMessage("null") // params left out
		}
		var v T
		if err := json.Unmarshal(m.Params, &v); err != nil {
			return out, fmt.Errorf("acptest: record %d, %s %s: %w", i, d, method, err)
		}
		out = append(out, v)
	}
	return out, nil
}
