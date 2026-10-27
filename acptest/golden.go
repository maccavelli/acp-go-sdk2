package acptest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
func ValidateGolden(tb testing.TB, records []Record) {
	tb.Helper()
	pending := map[Direction]map[string]string{Sent: {}, Received: {}}
	for i, r := range records {
		if err := validateRecord(r, pending); err != nil {
			tb.Errorf("acptest: record %d, %s: %v: %s", i, r.Direction, err, r.Frame)
		}
	}
}

// validateRecord checks one record, and keeps a request it holds in pending,
// by its direction and id, until its response.
func validateRecord(r Record, pending map[Direction]map[string]string) error {
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
			if hasID {
				pending[r.Direction][id] = method
			}
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
		if hasID {
			pending[r.Direction][id] = method
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
	method, ok := pending[asked][id]
	if !ok {
		return fmt.Errorf("a response to no recorded request: id %s", id)
	}
	delete(pending[asked], id)
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
