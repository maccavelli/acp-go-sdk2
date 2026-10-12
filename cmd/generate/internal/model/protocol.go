package model

import (
	"fmt"
	"slices"
	"strings"
)

// Protocol is the behaviour of ACP that the schema does not express. Build
// applies each rule only where the schema carries its subject, so a schema
// without the subject builds as if the rule were absent; it refuses a subject
// whose shape the rule cannot use. ACP, in acp.go, is the table the generator
// uses.
type Protocol struct {
	// Interfaces are the optional interfaces a method rule may bind to.
	Interfaces []Interface
	// Methods holds at most one rule per side and wire.
	Methods []MethodRule
	// HandWrittenHelpers lists the unions whose constructors helpers.go
	// writes; the generator emits none for them.
	HandWrittenHelpers []string
	// AbsolutePaths lists the properties the protocol requires to be absolute
	// paths, which Validate checks
	// (0015-MADR-validate-absolute-protocol-paths.md, W1).
	AbsolutePaths []PathRule
}

// PathRule names a property the protocol requires to be an absolute path,
// by the definition that declares it as the merged schema names it. Source
// is the specification sentence the rule follows.
type PathRule struct {
	Definition, Property, Source string
}

// Interface is a Go interface the dispatch code calls methods through.
type Interface struct {
	Binding Binding
	// Name is the interface; Var is the variable dispatch asserts it into,
	// empty for a required interface.
	Name, Var string
	Doc       []string
}

// MethodRule is what one wire method does beyond its schema.
type MethodRule struct {
	// Side is "agent" or "client"; Wire is the method on the wire.
	Side, Wire string
	// Binding, when set, is the optional interface the method is called
	// through instead of its side's required or experimental one.
	Binding Binding
	// GoName, when set, replaces the interface method's name.
	GoName string
	// PromptTurn: inbound, the method runs under a context its side's
	// CancelsPromptTurn method cancels; outbound, a call that fails with its
	// context done sends that method.
	PromptTurn bool
	// CancelsPromptTurn: inbound, the method first cancels the session's
	// prompt-turn context.
	CancelsPromptTurn bool
	// SessionKey is the property naming the session, for the two above.
	SessionKey string
	// StopKey is a prompt turn's result property that holds the stop reason,
	// a required string enum, and StopCancelled its value that answers a turn
	// whose context was cancelled
	// (0017-MADR-remediate-acp-v1-conformance-violations.md, P1).
	StopKey, StopCancelled string
	// CancelledByPromptCancel: a client request the client answers itself,
	// at once, when it sends the agent's prompt-turn cancel for the request's
	// session; CancelledKey is the result property, a required union, and
	// CancelledVariant the discriminator value of the variant that answers it
	// (0017-MADR-remediate-acp-v1-conformance-violations.md, Q1).
	CancelledByPromptCancel        bool
	CancelledKey, CancelledVariant string
	// Initializes marks the method that negotiates the connection: VersionKey
	// is the protocol-version property of its request and its result, a
	// required integer; RequestCapabilities and ResponseCapabilities the
	// properties of each side's capabilities, objects
	// (0017-MADR-remediate-acp-v1-conformance-violations.md, N1 and G1).
	Initializes                                           bool
	VersionKey, RequestCapabilities, ResponseCapabilities string
	// Capability is the dotted path, from AgentCapabilities or
	// ClientCapabilities, of the capability that gates the method. It is
	// recorded and checked, not emitted.
	Capability string
}

// sdkInterfaces are the interfaces every generated SDK has.
var sdkInterfaces = []Interface{
	{Binding: BindAgent, Name: "Agent"},
	{Binding: BindAgentExperimental, Name: "AgentExperimental", Var: "exp",
		Doc: []string{"AgentExperimental defines unstable methods that are not part of the official spec. These may change or be removed without notice."}},
	{Binding: BindClient, Name: "Client"},
	{Binding: BindClientExperimental, Name: "ClientExperimental", Var: "exp",
		Doc: []string{"ClientExperimental defines unstable methods that are not part of the official spec. These may change or be removed without notice."}},
}

// sideOf is the side whose methods a binding serves.
func sideOf(b Binding) string {
	switch b {
	case BindAgent, BindAgentOptional, BindAgentExperimental:
		return sideAgent
	case BindClient, BindClientExperimental:
		return sideClient
	default:
		return ""
	}
}

const (
	sideAgent    = "agent"
	sideClient   = "client"
	tablePrefix  = "protocol table: "
	optionalNone = BindUnknown
)

// rules is a Protocol checked on its own and indexed for Build.
type rules struct {
	methods    map[string]*MethodRule
	interfaces map[Binding]Interface
	helpers    map[string]bool
	table      *Protocol
}

func ruleKey(side, wire string) string { return side + "|" + wire }

func (r *MethodRule) owner() string { return fmt.Sprintf("%s method %q", r.Side, r.Wire) }

// newRules checks what a table says without the schema. A nil table has no
// rules.
func newRules(p *Protocol) (*rules, error) {
	r := &rules{methods: map[string]*MethodRule{}, interfaces: map[Binding]Interface{}, helpers: map[string]bool{}, table: p}
	if p == nil {
		return r, nil
	}
	for _, in := range p.Interfaces {
		switch {
		case in.Binding != BindAgentOptional:
			return nil, fmt.Errorf("interface %s: its binding must be optional", in.Name)
		case in.Var == "":
			return nil, fmt.Errorf("interface %s: it needs a variable name", in.Name)
		}
		if _, ok := r.interfaces[in.Binding]; ok {
			return nil, fmt.Errorf("interface %s: another interface has its binding", in.Name)
		}
		if err := checkExported(in.Name); err != nil {
			return nil, fmt.Errorf("interface %s: %w", in.Name, err)
		}
		r.interfaces[in.Binding] = in
	}
	for i := range p.Methods {
		rule := &p.Methods[i]
		if err := checkRule(rule, r); err != nil {
			return nil, fmt.Errorf("%s: %w", rule.owner(), err)
		}
		k := ruleKey(rule.Side, rule.Wire)
		if _, ok := r.methods[k]; ok {
			return nil, fmt.Errorf("%s: two rules", rule.owner())
		}
		r.methods[k] = rule
	}
	initializes := 0
	for _, rule := range p.Methods {
		if rule.Initializes {
			initializes++
		}
	}
	if initializes > 1 {
		return nil, fmt.Errorf("%d initialize rules; a protocol has one", initializes)
	}
	for _, name := range p.HandWrittenHelpers {
		r.helpers[name] = true
	}
	seen := map[[2]string]bool{}
	for _, rule := range p.AbsolutePaths {
		k := [2]string{rule.Definition, rule.Property}
		switch {
		case rule.Definition == "" || rule.Property == "" || rule.Source == "":
			return nil, fmt.Errorf("absolute-path rule %q.%q: it needs a definition, a property, and a source", rule.Definition, rule.Property)
		case seen[k]:
			return nil, fmt.Errorf("absolute-path rule %s.%s: two rules", rule.Definition, rule.Property)
		}
		seen[k] = true
	}
	return r, nil
}

// checkRule checks one method rule against the table's interfaces.
func checkRule(rule *MethodRule, r *rules) error {
	switch {
	case rule.Side != sideAgent && rule.Side != sideClient:
		return fmt.Errorf(`its side must be "agent" or "client"`)
	case rule.Binding != optionalNone && r.interfaces[rule.Binding].Name == "":
		return fmt.Errorf("no interface of the table has its binding")
	case rule.Binding != optionalNone && sideOf(rule.Binding) != rule.Side:
		return fmt.Errorf("it binds to an %s interface", sideOf(rule.Binding))
	}
	if err := checkRoles(rule); err != nil {
		return err
	}
	if rule.GoName != "" {
		if err := checkExported(rule.GoName); err != nil {
			return err
		}
	}
	if rule.Capability != "" && slices.Contains(strings.Split(rule.Capability, "."), "") {
		return fmt.Errorf("capability %q has an empty segment", rule.Capability)
	}
	return nil
}

// checkRoles checks the roles a rule gives its method: a prompt turn or its
// cancel, a request answered on that cancel, the initialize method; each on
// its side, with its keys, and no key without its role.
func checkRoles(rule *MethodRule) error {
	switch {
	case (rule.PromptTurn || rule.CancelsPromptTurn) && rule.Side != sideAgent:
		return fmt.Errorf("only agent methods are a prompt turn or its cancel")
	case rule.CancelledByPromptCancel && rule.Side != sideClient:
		return fmt.Errorf("only client methods are answered on a prompt-turn cancel")
	case rule.PromptTurn && rule.CancelsPromptTurn:
		return fmt.Errorf("it cannot be both a prompt turn and its cancel")
	case (rule.PromptTurn || rule.CancelsPromptTurn || rule.CancelledByPromptCancel) && rule.SessionKey == "":
		return fmt.Errorf("a prompt-turn role needs a session key")
	case !rule.PromptTurn && !rule.CancelsPromptTurn && !rule.CancelledByPromptCancel && rule.SessionKey != "":
		return fmt.Errorf("a session key without a prompt-turn role")
	case rule.CancelledByPromptCancel && (rule.CancelledKey == "" || rule.CancelledVariant == ""):
		return fmt.Errorf("a request answered on a prompt-turn cancel needs its result key and variant")
	case !rule.CancelledByPromptCancel && (rule.CancelledKey != "" || rule.CancelledVariant != ""):
		return fmt.Errorf("a cancelled key without a request answered on a prompt-turn cancel")
	case rule.Initializes && rule.Side != sideAgent:
		return fmt.Errorf("only an agent method initializes the connection")
	case rule.Initializes && (rule.VersionKey == "" || rule.RequestCapabilities == "" || rule.ResponseCapabilities == ""):
		return fmt.Errorf("an initialize rule needs its version key and both capabilities keys")
	case !rule.Initializes && (rule.VersionKey != "" || rule.RequestCapabilities != "" || rule.ResponseCapabilities != ""):
		return fmt.Errorf("a version or capabilities key without an initialize rule")
	case rule.PromptTurn && (rule.StopKey == "" || rule.StopCancelled == ""):
		return fmt.Errorf("a prompt turn needs its stop key and cancelled value")
	case !rule.PromptTurn && (rule.StopKey != "" || rule.StopCancelled != ""):
		return fmt.Errorf("a stop key without a prompt turn")
	}
	return nil
}

// applyMethodRule sets what a rule says on one built method.
func applyMethodRule(mi *Method, rule *MethodRule) {
	if rule == nil {
		return
	}
	if rule.Binding != optionalNone {
		mi.Binding = rule.Binding
	}
	if rule.GoName != "" {
		mi.GoName = rule.GoName
	}
	mi.PromptTurn, mi.CancelsPromptTurn, mi.Capability = rule.PromptTurn, rule.CancelsPromptTurn, rule.Capability
	mi.CancelledByPromptCancel, mi.Initializes = rule.CancelledByPromptCancel, rule.Initializes
}

// linkPromptTurns checks each prompt-turn method and cancel against the
// schema, records the Go field of its session key, and links each prompt
// turn to its side's one cancel.
func (m *Model) linkPromptTurns(r *rules) error {
	for _, side := range [][]*Method{m.AgentMethods, m.ClientMethods} {
		var cancels []*Method
		for _, mi := range side {
			if !mi.PromptTurn && !mi.CancelsPromptTurn {
				continue
			}
			rule := r.methods[ruleKey(mi.Side, mi.Wire)]
			typeName := mi.Req
			switch {
			case mi.PromptTurn && (mi.Notif != "" || !mi.HasResult):
				return fmt.Errorf("%s: a prompt turn must be a request with a result", rule.owner())
			case mi.CancelsPromptTurn && mi.Notif == "":
				return fmt.Errorf("%s: a prompt-turn cancel must be a notification", rule.owner())
			case mi.CancelsPromptTurn:
				typeName = mi.Notif
				cancels = append(cancels, mi)
			}
			f, err := m.sessionField(typeName, rule.SessionKey)
			if err != nil {
				return fmt.Errorf("%s: %w", rule.owner(), err)
			}
			mi.SessionField = f.Name
			if mi.PromptTurn {
				if mi.StopField, mi.StopConst, err = m.stopField(mi.Resp, rule.StopKey, rule.StopCancelled); err != nil {
					return fmt.Errorf("%s: %w", rule.owner(), err)
				}
			}
		}
		for _, mi := range side {
			if !mi.PromptTurn {
				continue
			}
			rule := r.methods[ruleKey(mi.Side, mi.Wire)]
			if len(cancels) != 1 {
				return fmt.Errorf("%s: a prompt turn needs one prompt-turn cancel on its side, found %d", rule.owner(), len(cancels))
			}
			c := cancels[0]
			mt, ct := m.fieldType(mi.Req, mi.SessionField), m.fieldType(c.Notif, c.SessionField)
			if mi.SessionField != c.SessionField || mt != ct {
				return fmt.Errorf("%s: its cancel %s keys the session by %s %s, not %s %s", rule.owner(), c.Wire, c.SessionField, ct, mi.SessionField, mt)
			}
			mi.Canceller = c
		}
	}
	return nil
}

// sessionField returns the field of a required string property of an object
// type.
func (m *Model) sessionField(typeName, key string) (*Field, error) {
	f := m.objectField(typeName, key)
	if f == nil || !f.Required {
		return nil, fmt.Errorf("%q is not a required property of %s", key, typeName)
	}
	if !m.isString(f.Type) {
		return nil, fmt.Errorf("%q of %s is not a string", key, typeName)
	}
	return f, nil
}

// linkInitialize checks the method that negotiates the connection, a request
// with a result, and records the Go fields of its version and of each side's
// capabilities; Model.Initialize names it.
func (m *Model) linkInitialize(r *rules) error {
	for _, mi := range m.AgentMethods {
		if !mi.Initializes {
			continue
		}
		rule := r.methods[ruleKey(mi.Side, mi.Wire)]
		if mi.Notif != "" || !mi.HasResult {
			return fmt.Errorf("%s: an initialize method must be a request with a result", rule.owner())
		}
		for _, f := range []struct {
			typeName, key string
			into          *string
		}{{mi.Req, rule.VersionKey, &mi.ReqVersionField}, {mi.Resp, rule.VersionKey, &mi.RespVersionField}} {
			field := m.objectField(f.typeName, f.key)
			if field == nil || !field.Required || !m.isInteger(field.Type) {
				return fmt.Errorf("%s: %q is not a required integer property of %s", rule.owner(), f.key, f.typeName)
			}
			*f.into = field.Name
		}
		for _, f := range []struct {
			typeName, key string
			into          *string
		}{{mi.Req, rule.RequestCapabilities, &mi.ReqCapsField}, {mi.Resp, rule.ResponseCapabilities, &mi.RespCapsField}} {
			field := m.objectField(f.typeName, f.key)
			if field == nil || field.Type.Kind != RefNamed || m.Type(field.Type.Name) == nil || m.Type(field.Type.Name).Kind != KindObject {
				return fmt.Errorf("%s: %q is not an object property of %s", rule.owner(), f.key, f.typeName)
			}
			*f.into = field.Name
		}
		m.Initialize = mi
	}
	return nil
}

// isInteger reports whether r is a Go integer type or a definition whose Go
// type is one.
func (m *Model) isInteger(r TypeRef) bool {
	if r.Kind != RefNamed {
		return false
	}
	name := r.Name
	if t := m.Type(name); t != nil && t.Kind == KindPrimitive && t.Underlying != nil {
		name = t.Underlying.Name
	}
	for _, g := range integerFormats {
		if g == name {
			return true
		}
	}
	return false
}

// linkCancelledRequests checks each request the client answers itself when
// it cancels a prompt turn: a request with a result, keyed by a session,
// whose result holds a union with the answering variant, built by a
// constructor without parameters; and marks the agent's prompt-turn cancel,
// whose client wrapper answers them.
func (m *Model) linkCancelledRequests(r *rules) error {
	var cancel *Method
	for _, mi := range m.AgentMethods {
		if mi.CancelsPromptTurn {
			cancel = mi
		}
	}
	for _, mi := range m.ClientMethods {
		if !mi.CancelledByPromptCancel {
			continue
		}
		rule := r.methods[ruleKey(mi.Side, mi.Wire)]
		switch {
		case mi.Notif != "" || !mi.HasResult:
			return fmt.Errorf("%s: a request answered on a prompt-turn cancel must be a request with a result", rule.owner())
		case cancel == nil:
			return fmt.Errorf("%s: the agent side has no prompt-turn cancel", rule.owner())
		}
		f, err := m.sessionField(mi.Req, rule.SessionKey)
		if err != nil {
			return fmt.Errorf("%s: %w", rule.owner(), err)
		}
		mi.SessionField = f.Name
		if mi.CancelledField, mi.CancelledHelper, err = m.cancelledAnswer(mi.Resp, rule.CancelledKey, rule.CancelledVariant); err != nil {
			return fmt.Errorf("%s: %w", rule.owner(), err)
		}
		cancel.CancelsPending = true
	}
	return nil
}

// cancelledAnswer returns the Go field of a result's union property and the
// constructor, without parameters, of its variant with the given
// discriminator value.
func (m *Model) cancelledAnswer(typeName, key, variant string) (field, helper string, err error) {
	f := m.objectField(typeName, key)
	if f == nil || !f.Required || f.Type.Kind != RefNamed {
		return "", "", fmt.Errorf("%q is not a required property of %s naming a union", key, typeName)
	}
	u := m.Type(f.Type.Name)
	if u == nil || u.Kind != KindUnion {
		return "", "", fmt.Errorf("%q of %s is not a union", key, typeName)
	}
	for _, h := range u.Helpers {
		if u.Variants[h.Variant].DiscValue != variant {
			continue
		}
		if h.Value || len(h.Params) != 0 {
			return "", "", fmt.Errorf("%s's constructor %s takes parameters", u.Name, h.Name)
		}
		return f.Name, h.Name, nil
	}
	return "", "", fmt.Errorf("%s has no constructor for a variant %q", u.Name, variant)
}

// stopField returns the Go field of a prompt turn's stop property, which
// must be a required property of the result whose type is a string enum,
// and the constant of its cancelled value.
func (m *Model) stopField(typeName, key, value string) (field, constant string, err error) {
	f := m.objectField(typeName, key)
	if f == nil || !f.Required || f.Type.Kind != RefNamed {
		return "", "", fmt.Errorf("%q is not a required property of %s naming a string enum", key, typeName)
	}
	t := m.Type(f.Type.Name)
	if t == nil || (t.Kind != KindEnum && t.Kind != KindStringConstUnion && t.Kind != KindOpenEnum) {
		return "", "", fmt.Errorf("%q of %s is not a string enum", key, typeName)
	}
	for _, ev := range t.Enum {
		if ev.Value == value {
			return f.Name, ev.Const, nil
		}
	}
	return "", "", fmt.Errorf("%q of %s has no value %q", key, typeName, value)
}

// objectField returns the field of an object type with the given key, or nil.
func (m *Model) objectField(typeName, key string) *Field {
	t := m.Type(typeName)
	if t == nil || t.Kind != KindObject {
		return nil
	}
	for _, f := range t.Fields {
		if f.Key == key {
			return f
		}
	}
	return nil
}

// fieldType is the Go type of an object type's field, by Go name.
func (m *Model) fieldType(typeName, field string) string {
	for _, f := range m.Type(typeName).Fields {
		if f.Name == field {
			return f.Type.String()
		}
	}
	return ""
}

// isString reports whether r is string or a definition whose Go type is.
func (m *Model) isString(r TypeRef) bool {
	if r.Kind != RefNamed {
		return false
	}
	if r.Name == schemaTypeString {
		return true
	}
	t := m.Type(r.Name)
	return t != nil && t.Kind == KindPrimitive && t.Underlying != nil && t.Underlying.Name == schemaTypeString
}

// checkCapabilities resolves each method's capability path from its side's
// capabilities type, through the named types of its fields.
func (m *Model) checkCapabilities(r *rules) error {
	for _, mi := range slices.Concat(m.AgentMethods, m.ClientMethods) {
		if mi.Capability == "" {
			continue
		}
		rule := r.methods[ruleKey(mi.Side, mi.Wire)]
		typeName := map[string]string{sideAgent: "AgentCapabilities", sideClient: "ClientCapabilities"}[mi.Side]
		segs := strings.Split(mi.Capability, ".")
		for i, seg := range segs {
			f := m.objectField(typeName, seg)
			if f == nil {
				return fmt.Errorf("%s: capability %q: %s has no property %q", rule.owner(), mi.Capability, typeName, seg)
			}
			if i == len(segs)-1 {
				break
			}
			ref := f.Type
			for ref.Kind == RefPointer {
				ref = *ref.Elem
			}
			typeName = ref.Name
		}
	}
	return nil
}

// checkTypeRules refuses a hand-written-helpers type that is not a union; a
// type the schema lacks is skipped.
func (m *Model) checkTypeRules(r *rules) error {
	if r.table == nil {
		return nil
	}
	for _, name := range r.table.HandWrittenHelpers {
		if t := m.Type(name); t != nil && t.Kind != KindUnion {
			return fmt.Errorf("hand-written helpers for %s: it is not a union", name)
		}
	}
	return nil
}

// InterfaceOf returns the interface a binding calls through, or the zero
// Interface for BindUnknown.
func (m *Model) InterfaceOf(b Binding) Interface {
	for _, in := range m.Interfaces {
		if in.Binding == b {
			return in
		}
	}
	return Interface{}
}

// buildInterfaces lists the SDK's interfaces and each table interface a
// method binds to, in binding order, declaring the table's names.
func (m *Model) buildInterfaces(n *namer, r *rules) error {
	m.Interfaces = slices.Clone(sdkInterfaces)
	used := map[Binding]bool{}
	for _, mi := range slices.Concat(m.AgentMethods, m.ClientMethods) {
		used[mi.Binding] = true
	}
	for _, b := range []Binding{BindAgentOptional} {
		in, ok := r.interfaces[b]
		if !ok || !used[b] {
			continue
		}
		if err := n.declare(in.Name, "the protocol table"); err != nil {
			return err
		}
		m.Interfaces = append(m.Interfaces, in)
	}
	slices.SortStableFunc(m.Interfaces, func(a, b Interface) int { return int(a.Binding) - int(b.Binding) })
	return nil
}

// interfaceName returns the name of the interface a binding calls through,
// before Interfaces is built.
func (m *Model) interfaceName(b Binding, r *rules) string {
	if in, ok := r.interfaces[b]; ok {
		return in.Name
	}
	for _, in := range sdkInterfaces {
		if in.Binding == b {
			return in.Name
		}
	}
	return ""
}
