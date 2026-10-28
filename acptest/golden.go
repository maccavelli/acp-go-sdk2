package acptest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// ValidateGolden fails tb for each record that is not a message the protocol
// defines, read as ConnectRecorded records a client's streams: a Sent frame
// went from the client to the agent, a Received one from the agent to the
// client. A record of an agent's own streams has the directions the other
// way round.
//
// Each request and notification is decoded into its method's params type and
// checked with its Validate, its method looked up by the way it went, since
// one method can name a different message each way; each response, a result
// or an error, is matched to the request it answers, by id, among those that
// went the other way, and a result is decoded into that method's result type
// and checked with its Validate. It fails a frame that is not a JSON-RPC 2.0
// message, an unknown method, a request sent as a notification or the other
// way round, a value its type refuses, and a response to no recorded request.
// An extension method's frame, one whose method begins with "_", is checked
// only as JSON-RPC, as its params are the extension's own. The canonical form
// is encoding/json v1's, as Recorder keeps it
// (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 8).
//
// A response is matched to its request wherever the request is among the
// records, before it or after it: the records of two taps, as
// agentproc.Transcript writes them, have no order between the two ways, so a
// response can be recorded before its request. Requests that share a way and
// an id are matched to their responses in order
// (0042-MADR-in-process-agent-runner.md, item 4).
func ValidateGolden(tb testing.TB, records []Record) {
	tb.Helper()
	pending := requestsOf(records)
	for i, r := range records {
		if err := validateRecord(r, pending); err != nil {
			tb.Errorf("acptest: record %d, %s: %v: %s", i, r.Direction, err, r.Frame)
		}
	}
}

// requestsOf is the first of ValidateGolden's two passes: the method of each
// request among records, by the way it went and its id, in the records'
// order. Only a request validateRecord can pass is kept: an extension
// method's, or a method the way's table has as a request. Any other is
// refused there, and a response to it is a response to no recorded request.
func requestsOf(records []Record) map[Direction]map[string][]string {
	pending := map[Direction]map[string][]string{Sent: {}, Received: {}}
	for _, r := range records {
		if _, ok := pending[r.Direction]; !ok {
			continue
		}
		var m struct {
			ID     json.RawMessage `json:"id"`
			Method *string         `json:"method"`
		}
		if json.Unmarshal(r.Frame, &m) != nil || m.Method == nil || *m.Method == "" || m.ID == nil {
			continue
		}
		if types, known := methodTables[r.Direction][*m.Method]; !strings.HasPrefix(*m.Method, "_") && (!known || types.result == nil) {
			continue
		}
		id := string(bytes.TrimSpace(m.ID))
		pending[r.Direction][id] = append(pending[r.Direction][id], *m.Method)
	}
	return pending
}

// updateEnv names the environment variable that has Golden write its file:
// a variable, not a flag, since a test binary's own -update would collide
// with a flag this package defined (0042-MADR-in-process-agent-runner.md,
// item 4, W6).
const updateEnv = "ACPTEST_UPDATE"

// Golden compares records with the golden file at path, in the form
// WriteRecords writes, each frame in the canonical form Recorder keeps, so
// that the order of a frame's keys never changes the file. Equal, it passes.
// With ACPTEST_UPDATE=1 set it writes path instead. Otherwise it writes the
// records to tb.ArtifactDir() and fails tb, naming the first record that
// differs, or the reason path cannot be read. A golden file's CRLF line ends
// read as LF (0042-MADR-in-process-agent-runner.md, item 4).
func Golden(tb testing.TB, path string, records []Record) {
	tb.Helper()
	canon := make([]Record, len(records))
	for i, r := range records {
		canon[i] = Record{Direction: r.Direction, Frame: canonical(r.Frame)}
	}
	var got bytes.Buffer
	if err := WriteRecords(&got, canon); err != nil {
		tb.Errorf("acptest: writing the records: %v", err)
		return
	}
	if os.Getenv(updateEnv) == "1" {
		if err := os.WriteFile(path, got.Bytes(), 0o644); err != nil { //nolint:gosec // G306: a golden file is meant to be read.
			tb.Errorf("acptest: updating the golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path) //nolint:gosec // G304: the test names its golden file.
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
	if err == nil && bytes.Equal(want, got.Bytes()) {
		return
	}
	out := filepath.Join(tb.ArtifactDir(), filepath.Base(path))
	if werr := os.WriteFile(out, got.Bytes(), 0o600); werr != nil {
		tb.Errorf("acptest: writing the records to %s: %v", out, werr)
	}
	if err != nil {
		tb.Errorf("acptest: golden %s: %v; the records are in %s, and %s=1 writes the golden", path, err, out, updateEnv)
		return
	}
	i, g, w := firstDiff(got.Bytes(), want)
	tb.Errorf("acptest: record %d differs from the golden %s\n got: %s\nwant: %s\nthe records are in %s, and %s=1 rewrites the golden",
		i, path, g, w, out, updateEnv)
}

// firstDiff returns the index of the first line that differs between got and
// want, and the two lines, "(none)" for a line one of them lacks.
func firstDiff(got, want []byte) (int, string, string) {
	g := strings.Split(strings.TrimSuffix(string(got), "\n"), "\n")
	w := strings.Split(strings.TrimSuffix(string(want), "\n"), "\n")
	line := func(lines []string, i int) string {
		if i < len(lines) {
			return lines[i]
		}
		return "(none)"
	}
	i := 0
	for i < len(g) && i < len(w) && g[i] == w[i] {
		i++
	}
	return i, line(g, i), line(w, i)
}

// validateRecord checks one record. A response takes the first request of
// pending it answers, by the other way and its id.
func validateRecord(r Record, pending map[Direction]map[string][]string) error {
	table, ok := methodTables[r.Direction]
	if !ok {
		return fmt.Errorf("direction %q is neither %q nor %q", r.Direction, Sent, Received)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(r.Frame, &m); err != nil {
		return fmt.Errorf("not a JSON object: %w", err)
	}
	var version string
	if err := json.Unmarshal(m["jsonrpc"], &version); err != nil || version != "2.0" {
		return errors.New(`"jsonrpc" is not "2.0"`)
	}
	rawID, hasID := m["id"]
	id := string(bytes.TrimSpace(rawID))
	if raw, ok := m["method"]; ok {
		var method string
		if err := json.Unmarshal(raw, &method); err != nil || method == "" {
			return errors.New("the method is not a non-empty string")
		}
		if strings.HasPrefix(method, "_") {
			return nil
		}
		types, known := table[method]
		switch {
		case !known:
			return fmt.Errorf("unknown method %q, %s", method, r.Direction)
		case hasID && types.result == nil:
			return fmt.Errorf("%s is a notification, and was sent as a request", method)
		case !hasID && types.result != nil:
			return fmt.Errorf("%s is a request, and was sent without an id", method)
		}
		params, ok := m["params"]
		if !ok {
			params = json.RawMessage("null")
		}
		if err := types.params(params); err != nil {
			return fmt.Errorf("%s params: %w", method, err)
		}
		return nil
	}
	if !hasID {
		return errors.New("neither a request, a notification nor a response")
	}
	asked := Sent
	if r.Direction == Sent {
		asked = Received
	}
	queue := pending[asked][id]
	if len(queue) == 0 {
		return fmt.Errorf("a response to no recorded request: id %s", id)
	}
	method := queue[0]
	pending[asked][id] = queue[1:]
	if raw, ok := m["error"]; ok {
		var e struct {
			Code    *int    `json:"code"`
			Message *string `json:"message"`
		}
		if err := json.Unmarshal(raw, &e); err != nil || e.Code == nil || e.Message == nil {
			return fmt.Errorf("the error answering %s has no integer code or string message", method)
		}
		return nil
	}
	result, ok := m["result"]
	if !ok {
		return fmt.Errorf("the response to %s has neither a result nor an error", method)
	}
	if strings.HasPrefix(method, "_") {
		return nil
	}
	if err := methodTables[asked][method].result(result); err != nil {
		return fmt.Errorf("%s result: %w", method, err)
	}
	return nil
}

// methodTypes checks a method's params and, for a request, its result.
type methodTypes struct {
	params func(json.RawMessage) error
	result func(json.RawMessage) error // nil for a notification
}

// checkValue decodes raw into a T and checks it with its Validate.
func checkValue[T any, PT interface {
	*T
	Validate() error
}](raw json.RawMessage) error {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	return PT(&v).Validate()
}

// request is the types of a request: params P, result R.
func request[P, R any, PP interface {
	*P
	Validate() error
}, PR interface {
	*R
	Validate() error
}]() methodTypes {
	return methodTypes{params: checkValue[P, PP], result: checkValue[R, PR]}
}

// notification is the types of a notification: params P.
func notification[P any, PP interface {
	*P
	Validate() error
}]() methodTypes {
	return methodTypes{params: checkValue[P, PP]}
}

// methodTables holds, by the way a frame goes, the types of each method that
// goes so: Sent, to the agent; Received, to the client. TestMethodTables
// requires every method constant of package acp in one of them.
var methodTables = map[Direction]map[string]methodTypes{
	Sent: {
		acp.AgentMethodAuthenticate:           request[acp.AuthenticateRequest, acp.AuthenticateResponse](),
		acp.AgentMethodInitialize:             request[acp.InitializeRequest, acp.InitializeResponse](),
		acp.AgentMethodLogout:                 request[acp.LogoutRequest, acp.LogoutResponse](),
		acp.AgentMethodSessionCancel:          notification[acp.CancelNotification](),
		acp.AgentMethodSessionClose:           request[acp.CloseSessionRequest, acp.CloseSessionResponse](),
		acp.AgentMethodSessionDelete:          request[acp.DeleteSessionRequest, acp.DeleteSessionResponse](),
		acp.AgentMethodSessionList:            request[acp.ListSessionsRequest, acp.ListSessionsResponse](),
		acp.AgentMethodSessionLoad:            request[acp.LoadSessionRequest, acp.LoadSessionResponse](),
		acp.AgentMethodSessionNew:             request[acp.NewSessionRequest, acp.NewSessionResponse](),
		acp.AgentMethodSessionPrompt:          request[acp.PromptRequest, acp.PromptResponse](),
		acp.AgentMethodSessionResume:          request[acp.ResumeSessionRequest, acp.ResumeSessionResponse](),
		acp.AgentMethodSessionSetConfigOption: request[acp.SetSessionConfigOptionRequest, acp.SetSessionConfigOptionResponse](),
		acp.AgentMethodSessionSetMode:         request[acp.SetSessionModeRequest, acp.SetSessionModeResponse](),
		acp.ProtocolMethodCancelRequest:       notification[acp.CancelRequestNotification](),
		// The unstable schema's methods, whose types are deprecated under v1
		// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md).
		acp.AgentMethodDocumentDidChange: notification[acp.UnstableDidChangeDocumentNotification](),                          //nolint:staticcheck // SA1019: an unstable method's own type.
		acp.AgentMethodDocumentDidClose:  notification[acp.UnstableDidCloseDocumentNotification](),                           //nolint:staticcheck // SA1019: an unstable method's own type.
		acp.AgentMethodDocumentDidFocus:  notification[acp.UnstableDidFocusDocumentNotification](),                           //nolint:staticcheck // SA1019: an unstable method's own type.
		acp.AgentMethodDocumentDidOpen:   notification[acp.UnstableDidOpenDocumentNotification](),                            //nolint:staticcheck // SA1019: an unstable method's own type.
		acp.AgentMethodDocumentDidSave:   notification[acp.UnstableDidSaveDocumentNotification](),                            //nolint:staticcheck // SA1019: an unstable method's own type.
		acp.AgentMethodMcpMessage:        notification[acp.UnstableMessageMcpNotification](),                                 //nolint:staticcheck // SA1019: an unstable method's own type.
		acp.AgentMethodNesAccept:         notification[acp.UnstableAcceptNesNotification](),                                  //nolint:staticcheck // SA1019: an unstable method's own type.
		acp.AgentMethodNesClose:          request[acp.UnstableCloseNesRequest, acp.UnstableCloseNesResponse](),               //nolint:staticcheck // SA1019: an unstable method's own types.
		acp.AgentMethodNesReject:         notification[acp.UnstableRejectNesNotification](),                                  //nolint:staticcheck // SA1019: an unstable method's own type.
		acp.AgentMethodNesStart:          request[acp.UnstableStartNesRequest, acp.UnstableStartNesResponse](),               //nolint:staticcheck // SA1019: an unstable method's own types.
		acp.AgentMethodNesSuggest:        request[acp.UnstableSuggestNesRequest, acp.UnstableSuggestNesResponse](),           //nolint:staticcheck // SA1019: an unstable method's own types.
		acp.AgentMethodProvidersDisable:  request[acp.UnstableDisableProviderRequest, acp.UnstableDisableProviderResponse](), //nolint:staticcheck // SA1019: an unstable method's own types.
		acp.AgentMethodProvidersList:     request[acp.UnstableListProvidersRequest, acp.UnstableListProvidersResponse](),     //nolint:staticcheck // SA1019: an unstable method's own types.
		acp.AgentMethodProvidersSet:      request[acp.UnstableSetProviderRequest, acp.UnstableSetProviderResponse](),         //nolint:staticcheck // SA1019: an unstable method's own types.
		acp.AgentMethodSessionFork:       request[acp.UnstableForkSessionRequest, acp.UnstableForkSessionResponse](),         //nolint:staticcheck // SA1019: an unstable method's own types.
	},
	Received: {
		acp.ClientMethodElicitationComplete:      notification[acp.CompleteElicitationNotification](),
		acp.ClientMethodElicitationCreate:        request[acp.CreateElicitationRequest, acp.CreateElicitationResponse](),
		acp.ClientMethodFsReadTextFile:           request[acp.ReadTextFileRequest, acp.ReadTextFileResponse](),
		acp.ClientMethodFsWriteTextFile:          request[acp.WriteTextFileRequest, acp.WriteTextFileResponse](),
		acp.ClientMethodSessionRequestPermission: request[acp.RequestPermissionRequest, acp.RequestPermissionResponse](),
		acp.ClientMethodSessionUpdate:            notification[acp.SessionNotification](),
		acp.ClientMethodTerminalCreate:           request[acp.CreateTerminalRequest, acp.CreateTerminalResponse](),
		acp.ClientMethodTerminalKill:             request[acp.KillTerminalRequest, acp.KillTerminalResponse](),
		acp.ClientMethodTerminalOutput:           request[acp.TerminalOutputRequest, acp.TerminalOutputResponse](),
		acp.ClientMethodTerminalRelease:          request[acp.ReleaseTerminalRequest, acp.ReleaseTerminalResponse](),
		acp.ClientMethodTerminalWaitForExit:      request[acp.WaitForTerminalExitRequest, acp.WaitForTerminalExitResponse](),
		acp.ProtocolMethodCancelRequest:          notification[acp.CancelRequestNotification](),
		acp.ClientMethodMcpMessage:               request[acp.UnstableMessageMcpRequest, acp.UnstableMessageMcpResponse](), //nolint:staticcheck // SA1019: an unstable method's own types.
	},
}
