package model

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

// Binding is the interface a method is called through.
type Binding int

const (
	// BindUnknown is no generated interface: a protocol method.
	BindUnknown Binding = iota
	// BindAgent places the method on the Agent interface.
	BindAgent
	// BindAgentOptional places the method on the optional agent interface
	// the protocol table declares.
	BindAgentOptional
	// BindAgentExperimental places the method on the AgentExperimental interface.
	BindAgentExperimental
	// BindClient places the method on the Client interface.
	BindClient
	// BindClientExperimental places the method on the ClientExperimental interface.
	BindClientExperimental
)

// Method is one wire method of the meta files and its Go types.
type Method struct {
	// Side is "agent", "client", or "protocol"; Wire is the method name on
	// the wire; Key is its meta key; Const is its constant, empty for a
	// protocol method.
	Side, Wire, Key, Const string
	// Req, Resp, and Notif are the Go types linked to the method by x-method
	// and x-side.
	Req, Resp, Notif string
	// GoName is the interface method's name.
	GoName string
	// HasResult is true for a request whose response is not null.
	HasResult bool
	Binding   Binding
	// Unstable is true when the wire is only in the unstable meta.
	Unstable bool
	// DocsIgnored is true when a linked type is x-docs-ignore.
	DocsIgnored bool
	// Doc is the interface method's doc comment.
	Doc []string
	// PromptTurn and CancelsPromptTurn are the method's prompt-turn role
	// from the protocol table; SessionField is the Go field of its session
	// key; Canceller is a prompt turn's cancel.
	PromptTurn, CancelsPromptTurn bool
	SessionField                  string
	Canceller                     *Method
	// StopField and StopConst are a prompt turn's stop-reason field and the
	// constant that answers a cancelled turn.
	StopField, StopConst string
	// CancelledByPromptCancel marks a client request the client answers itself
	// when it cancels the request's session's prompt turn, with the variant
	// CancelledHelper builds in its result's CancelledField; CancelsPending marks
	// the agent's prompt-turn cancel when the client has such requests.
	CancelledByPromptCancel, CancelsPending bool
	CancelledField, CancelledHelper         string
	// Initializes marks the method that negotiates the connection, with the
	// Go fields of the version in its request and result, and of each side's
	// capabilities.
	Initializes                                                    bool
	ReqVersionField, RespVersionField, ReqCapsField, RespCapsField string
	// Capability is the capability path that gates the method, from the
	// protocol table; CapabilitySteps are its Go fields from the side's
	// capabilities type, each marked when it is a pointer, the last a bool
	// or a pointer whose presence is the advertisement.
	Capability      string
	CapabilitySteps []CapabilityStep
}

// CapabilityStep is one field of a capability path.
type CapabilityStep struct {
	Field   string
	Pointer bool
}

// roles holds the definitions x-method and x-side link to one method.
type roles struct{ req, resp, notif string }

// linkRoles groups the definitions that carry x-method and x-side by side and
// wire. The role is the name's suffix.
func linkRoles(schema *load.Schema) (map[string]*roles, error) {
	out := map[string]*roles{}
	for _, name := range slices.Sorted(maps.Keys(schema.Defs)) {
		def := schema.Defs[name]
		if def == nil || def.XMethod == "" || def.XSide == "" {
			continue
		}
		k := def.XSide + "|" + def.XMethod
		r := out[k]
		if r == nil {
			r = &roles{}
			out[k] = r
		}
		var slot *string
		switch {
		case strings.HasSuffix(name, "Request"):
			slot = &r.req
		case strings.HasSuffix(name, "Response"):
			slot = &r.resp
		case strings.HasSuffix(name, "Notification"):
			slot = &r.notif
		default:
			return nil, fmt.Errorf("definition %s: linked to %s method %q, but its name ends in none of Request, Response, Notification", name, def.XSide, def.XMethod)
		}
		if *slot != "" {
			return nil, fmt.Errorf("definition %s: %s method %q already has %s in that role", name, def.XSide, def.XMethod, *slot)
		}
		*slot = name
	}
	return out, nil
}

// buildMethods resolves every method of the meta files and checks the
// methods of each interface for collisions.
func buildMethods(m *Model, prov *load.Provenance, r *rules) error {
	linked, err := linkRoles(m.schema)
	if err != nil {
		return err
	}
	for _, side := range []struct {
		name    string
		methods map[string]string
		consts  map[string]string
		out     *[]*Method
	}{
		{"agent", m.meta.AgentMethods, m.AgentMethodConsts, &m.AgentMethods},
		{"client", m.meta.ClientMethods, m.ClientMethodConsts, &m.ClientMethods},
		{"protocol", m.meta.ProtocolMethods, m.ProtocolMethodConsts, &m.Protocol},
	} {
		for _, key := range slices.Sorted(maps.Keys(side.methods)) {
			mi, err := buildMethod(m.schema, linked, prov, side.name, key, side.methods[key])
			if err != nil {
				return err
			}
			mi.Const = side.consts[key]
			applyMethodRule(mi, r.methods[ruleKey(mi.Side, mi.Wire)])
			*side.out = append(*side.out, mi)
		}
	}

	seen := map[Binding]map[string]string{}
	for _, mi := range slices.Concat(m.AgentMethods, m.ClientMethods) {
		iface := m.interfaceName(mi.Binding, r)
		if err := checkExported(mi.GoName); err != nil {
			return fmt.Errorf("%s method %q: interface method name: %w", mi.Side, mi.Wire, err)
		}
		if seen[mi.Binding] == nil {
			seen[mi.Binding] = map[string]string{}
		}
		if prev, ok := seen[mi.Binding][mi.GoName]; ok {
			return fmt.Errorf("interface %s: methods %q and %q both become %s", iface, prev, mi.Wire, mi.GoName)
		}
		seen[mi.Binding][mi.GoName] = mi.Wire
	}
	return nil
}

func buildMethod(schema *load.Schema, linked map[string]*roles, prov *load.Provenance, side, key, wire string) (*Method, error) {
	owner := fmt.Sprintf("%s method %q", side, wire)
	r := linked[side+"|"+wire]
	switch {
	case r == nil || (r.req == "" && r.notif == ""):
		return nil, fmt.Errorf("%s: no request or notification definition is linked to it by x-method and x-side", owner)
	case r.req != "" && r.notif != "":
		return nil, fmt.Errorf("%s: both %s and %s are linked to it", owner, r.req, r.notif)
	case r.req != "" && r.resp == "":
		return nil, fmt.Errorf("%s: request %s has no linked response definition", owner, r.req)
	}
	mi := &Method{Side: side, Wire: wire, Key: key, Req: r.req, Resp: r.resp, Notif: r.notif}
	if r.notif != "" {
		mi.GoName = strings.TrimSuffix(r.notif, "Notification")
	} else {
		mi.GoName = strings.TrimSuffix(r.req, "Request")
		resp := schema.Defs[r.resp]
		mi.HasResult = resp == nil || resp.Type != schemaTypeNull
	}
	mi.Unstable = prov != nil && prov.UnstableWires[wire]
	mi.Binding = bindingOf(side, mi.Unstable)
	for _, name := range []string{r.req, r.resp, r.notif} {
		if d := schema.Defs[name]; name != "" && d != nil && d.DocsIgnore {
			mi.DocsIgnored = true
		}
	}
	if !mi.DocsIgnored {
		// Prefer the notification's description, then the request's, then the response's.
		for _, name := range []string{r.notif, r.req, r.resp} {
			if d := schema.Defs[name]; name != "" && d != nil && d.Description != "" {
				mi.Doc = docLines(d.Description)
				break
			}
		}
	}
	if mi.Unstable {
		mi.Doc = markUnstable(mi.Doc)
	}
	return mi, nil
}

// unstableNotice is the schema's own notice on unstable capabilities.
var unstableNotice = []string{"**UNSTABLE**", "", "This capability is not part of the spec yet, and may be removed or changed at any point."}

// markUnstable puts the unstable notice at the head of doc, separated from it
// by a blank line, unless doc already carries the notice.
func markUnstable(doc []string) []string {
	for _, line := range doc {
		if strings.Contains(line, unstableNotice[0]) {
			return doc
		}
	}
	out := slices.Clone(unstableNotice)
	if len(doc) > 0 {
		out = append(append(out, ""), doc...)
	}
	return out
}

// bindingOf places a method on its side's required interface, or, when
// unstable, on its experimental one. The protocol table may bind it elsewhere.
func bindingOf(side string, unstable bool) Binding {
	switch side {
	case sideAgent:
		if unstable {
			return BindAgentExperimental
		}
		return BindAgent
	case sideClient:
		if unstable {
			return BindClientExperimental
		}
		return BindClient
	default:
		return BindUnknown
	}
}
