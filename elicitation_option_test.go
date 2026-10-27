package acp_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
)

// answerClient answers every elicitation with what respond returns for it.
type answerClient struct {
	*acptest.MemoryClient
	respond func(acp.CreateElicitationRequest) acp.CreateElicitationResponse
}

func (c *answerClient) CreateElicitation(_ context.Context, req acp.CreateElicitationRequest) (acp.CreateElicitationResponse, error) {
	return c.respond(req), nil
}

// optionPair joins a ScriptAgent and client, each side with its options, and
// initializes them with both elicitation modes advertised. It records what the
// client writes, and logs the client's connection to the returned log.
func optionPair(t *testing.T, client acp.Client, agentOpts, clientOpts []acp.ConnectionOption) (acptest.Pair, *acptest.Recorder, *muxLog) {
	t.Helper()
	rec := &acptest.Recorder{}
	log := &muxLog{}
	agentReads, clientWrites := io.Pipe()
	clientReads, agentWrites := io.Pipe()
	pair := acptest.Pair{
		Agent: acp.NewAgentSideConnection(&acptest.ScriptAgent{}, agentWrites, agentReads, agentOpts...),
		Client: acp.NewClientSideConnection(client, rec.Writer(clientWrites), clientReads,
			slices.Concat(clientOpts, []acp.ConnectionOption{acp.WithLogger(slog.New(slog.NewTextHandler(log, nil)))})...),
	}
	t.Cleanup(func() {
		if err := pair.Close(); err != nil {
			t.Error(err)
		}
		if err := pair.Wait(); !errors.Is(err, acp.ErrConnectionClosed) {
			t.Error(err)
		}
	})
	caps := acp.ClientCapabilities{Elicitation: &acp.ElicitationCapabilities{Form: &acp.ElicitationFormCapabilities{}, Url: &acp.ElicitationUrlCapabilities{}}}
	if _, err := pair.Client.Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber, ClientCapabilities: caps}); err != nil {
		t.Fatal(err)
	}
	return pair, rec, log
}

// nameForm asks for one required name.
func nameForm(t *testing.T) acp.CreateElicitationRequest {
	t.Helper()
	req := acp.NewCreateElicitationRequestForm("Your name?", decodeSchema(t, `{"type":"object","required":["name"],"properties":{"name":{"type":"string","minLength":1}}}`))
	req.Form.Session = &acp.ElicitationSessionScope{SessionId: "s"}
	return req
}

// accept answers with content.
func accept(content map[string]acp.ElicitationContentValue) acp.CreateElicitationResponse {
	resp := acp.NewCreateElicitationResponseAccept()
	resp.Accept.Content = content
	return resp
}

var failing = func(acp.CreateElicitationRequest) acp.CreateElicitationResponse { return accept(nil) }

// TestElicitationCheckOptionClient refuses, on a client that sets the option,
// an answer that fails Check: nothing of it is written, the agent gets -32603
// naming the field and the rule, and the client's log says so.
func TestElicitationCheckOptionClient(t *testing.T) {
	t.Parallel()
	client := &answerClient{MemoryClient: &acptest.MemoryClient{}, respond: failing}
	pair, rec, log := optionPair(t, client, nil, []acp.ConnectionOption{acp.WithElicitationCheck()})

	_, err := pair.Agent.CreateElicitation(t.Context(), nameForm(t))
	re, ok := errors.AsType[*acp.RequestError](err)
	if !ok || re.Code != -32603 || !strings.Contains(re.Reason(), `content["name"]: required`) {
		t.Fatalf("the agent got %v, want a -32603 naming content[\"name\"] and required", err)
	}
	for _, f := range rec.Frames() {
		if strings.Contains(string(f), `"action"`) {
			t.Errorf("the client wrote an answer it should have refused: %s", f)
		}
	}
	if !strings.Contains(log.String(), "handler result fails validation; not sent") {
		t.Errorf("the client's log is %q, want the refusal", log.String())
	}
}

// TestElicitationCheckOptionAgent returns, on an agent that sets the option,
// an error for an answer that fails Check, and the zero response.
func TestElicitationCheckOptionAgent(t *testing.T) {
	t.Parallel()
	client := &answerClient{MemoryClient: &acptest.MemoryClient{}, respond: failing}
	pair, _, _ := optionPair(t, client, []acp.ConnectionOption{acp.WithElicitationCheck()}, nil)

	resp, err := pair.Agent.CreateElicitation(t.Context(), nameForm(t))
	ve, ok := errors.AsType[*acp.ValidationError](err)
	if !ok || ve.Rule != acp.RuleRequired || !strings.HasPrefix(err.Error(), "elicitation/create response: ") {
		t.Fatalf("the agent got %v, want an elicitation/create response error wrapping RuleRequired", err)
	}
	if resp.Accept != nil || resp.Decline != nil || resp.Cancel != nil || resp.Other != nil {
		t.Errorf("the agent got %+v with the error, want the zero response", resp)
	}
}

// TestElicitationCheckOptionPasses lets through, with the option on both
// sides, a passing accept, a decline, a cancel and a URL-mode accept; with
// the option on neither, a failing accept arrives unchecked.
func TestElicitationCheckOptionPasses(t *testing.T) {
	t.Parallel()
	urlReq := acp.NewCreateElicitationRequestUrl("Sign in", "e-1", "https://example.com/auth")
	urlReq.Url.Session = &acp.ElicitationSessionScope{SessionId: "s"}
	for _, tc := range []struct {
		name    string
		req     acp.CreateElicitationRequest
		respond func(acp.CreateElicitationRequest) acp.CreateElicitationResponse
		opts    []acp.ConnectionOption
		want    string
	}{
		{"a passing accept", nameForm(t), func(acp.CreateElicitationRequest) acp.CreateElicitationResponse {
			return accept(map[string]acp.ElicitationContentValue{"name": acp.NewElicitationContentValueString("Ann")})
		}, []acp.ConnectionOption{acp.WithElicitationCheck()}, `{"action":"accept","content":{"name":"Ann"}}`},
		{"a decline", nameForm(t), func(acp.CreateElicitationRequest) acp.CreateElicitationResponse {
			return acp.NewCreateElicitationResponseDecline()
		}, []acp.ConnectionOption{acp.WithElicitationCheck()}, `{"action":"decline"}`},
		{"a cancel", nameForm(t), func(acp.CreateElicitationRequest) acp.CreateElicitationResponse {
			return acp.NewCreateElicitationResponseCancel()
		}, []acp.ConnectionOption{acp.WithElicitationCheck()}, `{"action":"cancel"}`},
		{"a URL-mode accept", urlReq, failing, []acp.ConnectionOption{acp.WithElicitationCheck()}, `{"action":"accept"}`},
		{"no option, a failing accept", nameForm(t), failing, nil, `{"action":"accept"}`},
	} {
		client := &answerClient{MemoryClient: &acptest.MemoryClient{}, respond: tc.respond}
		pair, _, _ := optionPair(t, client, tc.opts, tc.opts)
		resp, err := pair.Agent.CreateElicitation(t.Context(), tc.req)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if b, err := json.Marshal(resp); err != nil || string(b) != tc.want {
			t.Errorf("%s: the agent got %s, %v; want %s", tc.name, b, err, tc.want)
		}
	}
}
