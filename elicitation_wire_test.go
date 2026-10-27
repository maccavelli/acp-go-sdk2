package acp_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
)

// formClient answers form elicitations from a script, as a client whose user
// types each field: one attempt a request, each mapping a field to its texts,
// filled through Fields, ParseValue and Check. An attempt that fails is given
// back to the script, as a client gives it back to its user, and the next is
// tried; a nil attempt declines.
type formClient struct {
	*acptest.MemoryClient
	mu       sync.Mutex
	attempts []map[string][]string
	refused  []error
}

func (c *formClient) CreateElicitation(_ context.Context, req acp.CreateElicitationRequest) (acp.CreateElicitationResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if req.Form == nil {
		return acp.NewCreateElicitationResponseCancel(), nil
	}
	schema := req.Form.RequestedSchema
	for len(c.attempts) > 0 {
		attempt := c.attempts[0]
		c.attempts = c.attempts[1:]
		if attempt == nil {
			return acp.NewCreateElicitationResponseDecline(), nil
		}
		content, err := fill(schema, attempt)
		if err == nil {
			err = schema.Check(content)
		}
		if err != nil {
			c.refused = append(c.refused, err)
			continue
		}
		resp := acp.NewCreateElicitationResponseAccept()
		resp.Accept.Content = content
		return resp, nil
	}
	return acp.NewCreateElicitationResponseCancel(), nil
}

// fill turns an attempt's texts into an answer, field by field.
func fill(schema acp.ElicitationSchema, attempt map[string][]string) (map[string]acp.ElicitationContentValue, error) {
	content := map[string]acp.ElicitationContentValue{}
	for _, f := range schema.Fields() {
		texts, ok := attempt[f.Name]
		if !ok {
			continue
		}
		v, err := f.Schema.ParseValue(texts...)
		if err != nil {
			return nil, err
		}
		content[f.Name] = v
	}
	return content, nil
}

// TestElicitationFormRoundTrip sends a form from an agent to a client over a
// pair: the client's first answer fails Check and goes back to its user, the
// second is sent, and the agent's own Check of it passes. A decline reaches
// the agent as a decline.
func TestElicitationFormRoundTrip(t *testing.T) {
	t.Parallel()
	client := &formClient{MemoryClient: &acptest.MemoryClient{}, attempts: []map[string][]string{
		{"name": {"Ann"}, "age": {"200"}},
		{"name": {"Ann"}, "age": {"42"}, "contact": {"ann@example.com"}, "topics": {"go", "acp"}},
		nil,
	}}
	pair := acptest.Connect(t, &acptest.ScriptAgent{}, client)
	ctx := t.Context()
	caps := acp.ClientCapabilities{Elicitation: &acp.ElicitationCapabilities{Form: &acp.ElicitationFormCapabilities{}}}
	if _, err := pair.Client.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber, ClientCapabilities: caps}); err != nil {
		t.Fatal(err)
	}

	schema := decodeSchema(t, `{"type":"object","required":["name","age"],"properties":{
		"name":{"type":"string","minLength":1},
		"age":{"type":"integer","minimum":0,"maximum":150},
		"contact":{"type":"string","format":"email"},
		"topics":{"type":"array","items":{"type":"string","enum":["go","acp","mcp"]},"maxItems":2}}}`)
	req := acp.NewCreateElicitationRequestForm("Tell us about you", schema)
	req.Form.Session = &acp.ElicitationSessionScope{SessionId: "s"}

	resp, err := pair.Agent.CreateElicitation(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Accept == nil {
		t.Fatalf("the agent got %+v, want accept", resp)
	}
	if err := req.Form.RequestedSchema.Check(resp.Accept.Content); err != nil {
		t.Errorf("the agent's Check of the answer it received: %v", err)
	}
	b, err := json.Marshal(resp.Accept.Content)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"age":42,"contact":"ann@example.com","name":"Ann","topics":["go","acp"]}`; string(b) != want {
		t.Errorf("the agent got %s, want %s", b, want)
	}
	client.mu.Lock()
	refused := client.refused
	client.mu.Unlock()
	if len(refused) != 1 {
		t.Errorf("the client refused %d attempts, want 1, the first: %v", len(refused), refused)
	} else if ve, ok := errors.AsType[*acp.ValidationError](refused[0]); !ok || ve.Rule != acp.RuleMaximum {
		t.Errorf("the client refused its first attempt with %v, want age's maximum", refused[0])
	}

	resp, err = pair.Agent.CreateElicitation(ctx, req)
	if err != nil || resp.Decline == nil {
		t.Errorf("the second elicitation got %+v, %v; want decline", resp, err)
	}
}
