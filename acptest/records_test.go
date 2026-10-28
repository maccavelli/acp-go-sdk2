package acptest_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
)

// update is a session/update notification for session id, as the agent
// sends it.
func update(id string) acptest.Record {
	return acptest.Record{Direction: acptest.Received, Frame: json.RawMessage(
		`{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"` + id +
			`","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"hi"}}}}`)}
}

// TestRecordsRoundTrip requires WriteRecords and ReadRecords to keep
// records, a frame that is not JSON as a JSON string, and ReadRecords to
// skip blank lines (0042-MADR-in-process-agent-runner.md, item 4).
func TestRecordsRoundTrip(t *testing.T) {
	t.Parallel()
	records := []acptest.Record{update("s"), {Direction: acptest.Sent, Frame: json.RawMessage("not json")}}
	var b bytes.Buffer
	if err := acptest.WriteRecords(&b, records); err != nil {
		t.Fatal(err)
	}
	got, err := acptest.ReadRecords(strings.NewReader(b.String() + "\n  \n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Direction != acptest.Received || !bytes.Equal(got[0].Frame, records[0].Frame) ||
		got[1].Direction != acptest.Sent || string(got[1].Frame) != `"not json"` {
		t.Errorf("ReadRecords = %s, want the records written, the second frame a JSON string", got)
	}
}

// TestReadRecordsRefuses requires ReadRecords to refuse a line that is not a
// record, or whose direction is unknown, naming the line.
func TestReadRecordsRefuses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ in, want string }{
		{`{"direction":"sent","frame":{}}` + "\nnot a record\n", "line 2"},
		{`{"direction":"sideways","frame":{}}` + "\n", `line 1: direction "sideways"`},
	} {
		if _, err := acptest.ReadRecords(strings.NewReader(tc.in)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("ReadRecords(%q) = %v, want an error naming %q", tc.in, err, tc.want)
		}
	}
}

// TestDecode requires Decode to return the params of the records that went
// its way with its method, and to refuse params its type refuses, naming the
// record.
func TestDecode(t *testing.T) {
	t.Parallel()
	sentUpdate := update("sent-way")
	sentUpdate.Direction = acptest.Sent
	records := []acptest.Record{update("a"), sentUpdate, {Direction: acptest.Received, Frame: json.RawMessage(
		`{"jsonrpc":"2.0","id":1,"result":{}}`)}, update("b")}
	got, err := acptest.Decode[acp.SessionNotification](records, acptest.Received, acp.ClientMethodSessionUpdate)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].SessionId != "a" || got[1].SessionId != "b" {
		t.Errorf("Decode = %+v, want the received updates of a and b", got)
	}
	bad := append(records, acptest.Record{Direction: acptest.Received, Frame: json.RawMessage(
		`{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":7}}`)})
	if _, err := acptest.Decode[acp.SessionNotification](bad, acptest.Received, acp.ClientMethodSessionUpdate); err == nil ||
		!strings.Contains(err.Error(), "record 4") {
		t.Errorf("Decode = %v, want an error naming record 4", err)
	}
}

// TestValidateGoldenResponseFirst requires ValidateGolden to match a response
// recorded before its request, and requests sharing an id to their responses
// in order; and a response to a refused request to be refused, not to panic.
func TestValidateGoldenResponseFirst(t *testing.T) {
	t.Parallel()
	records := []acptest.Record{
		{Direction: acptest.Received, Frame: json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1}}`)},
		{Direction: acptest.Sent, Frame: json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1,"clientCapabilities":{}}}`)},
		{Direction: acptest.Sent, Frame: json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"_x/a","params":{}}`)},
		{Direction: acptest.Received, Frame: json.RawMessage(`{"jsonrpc":"2.0","id":2,"result":{}}`)},
		{Direction: acptest.Sent, Frame: json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"_x/b","params":{}}`)},
		{Direction: acptest.Received, Frame: json.RawMessage(`{"jsonrpc":"2.0","id":2,"result":{}}`)},
	}
	tb := &errorsTB{TB: t}
	acptest.ValidateGolden(tb, records)
	if len(tb.errs) != 0 {
		t.Errorf("ValidateGolden refused %q, want a response recorded first, and a reused id, to pass", tb.errs)
	}
	refused := []acptest.Record{
		{Direction: acptest.Received, Frame: json.RawMessage(`{"jsonrpc":"2.0","id":3,"method":"session/update","params":{}}`)},
		{Direction: acptest.Sent, Frame: json.RawMessage(`{"jsonrpc":"2.0","id":3,"result":{}}`)},
	}
	tb = &errorsTB{TB: t}
	acptest.ValidateGolden(tb, refused)
	if len(tb.errs) != 2 || !strings.Contains(tb.errs[1], "a response to no recorded request") {
		t.Errorf("ValidateGolden = %q, want the notification sent as a request and its response refused", tb.errs)
	}
}

// TestGolden requires Golden to write its file under ACPTEST_UPDATE=1, to
// pass records equal to it whatever their keys' order, and to fail a
// changed record or a missing file, writing the records to the test's
// artifact directory.
func TestGolden(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.golden")
	records := []acptest.Record{update("s"), update("t")}
	t.Setenv("ACPTEST_UPDATE", "1")
	acptest.Golden(t, path, records)
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the update wrote no golden: %v", err)
	}
	t.Setenv("ACPTEST_UPDATE", "")

	reordered := []acptest.Record{update("s"), {Direction: acptest.Received, Frame: json.RawMessage(
		`{"params":{"update":{"content":{"text":"hi","type":"text"},"sessionUpdate":"agent_message_chunk"},"sessionId":"t"},"method":"session/update","jsonrpc":"2.0"}`)}}
	tb := &errorsTB{TB: t}
	acptest.Golden(tb, path, reordered)
	if len(tb.errs) != 0 {
		t.Errorf("Golden refused equal records: %q", tb.errs)
	}

	changed := []acptest.Record{update("s"), update("u")}
	tb = &errorsTB{TB: t}
	acptest.Golden(tb, path, changed)
	if len(tb.errs) != 1 || !strings.Contains(tb.errs[0], "record 1 differs") {
		t.Errorf("Golden = %q, want record 1 named", tb.errs)
	}
	artifact, err := os.ReadFile(filepath.Join(t.ArtifactDir(), "session.golden"))
	if err != nil || !bytes.Contains(artifact, []byte(`"u"`)) {
		t.Errorf("the artifact holds %q, %v; want the changed records", artifact, err)
	}
	if now, _ := os.ReadFile(path); !bytes.Equal(now, written) {
		t.Error("Golden changed the golden without ACPTEST_UPDATE")
	}

	tb = &errorsTB{TB: t}
	acptest.Golden(tb, filepath.Join(t.TempDir(), "missing.golden"), records)
	if len(tb.errs) != 1 || !strings.Contains(tb.errs[0], "ACPTEST_UPDATE=1 writes the golden") {
		t.Errorf("Golden = %q, want the missing file reported", tb.errs)
	}
}
