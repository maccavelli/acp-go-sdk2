package emit

import (
	"go/ast"
	"go/token"
	"slices"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// sessionKey is string(p.<field>), the session's key in sessionCancels.
func sessionKey(field string) ast.Expr { return call(id("string"), dot("p", field)) }

// lockCall is a.mu.Lock() or a.mu.Unlock().
func lockCall(name string) ast.Stmt { return exprStmt(call(dot("a", "mu", name))) }

// handleFunc is func (recv *recvType) handle(ctx, method, params) (any, *RequestError) { switch method { ... } }.
func handleFunc(f *goFile, recv, recvType string, cases []ast.Stmt) *ast.FuncDecl {
	cases = append(cases, caseOf(nil, methodNotFound()))
	return method(param(recv, star(id(recvType))), "handle",
		funcType(
			[]*ast.Field{param("ctx", f.qual(pkgContext, "Context")), param("method", id("string")), param("params", f.qual(pkgJSON, "RawMessage"))},
			param("", id("any")), param("", star(id("RequestError"))),
		),
		switchStmt(id("method"), cases...),
	)
}

// promptCase runs a prompt turn under a context its session's cancel, a
// later prompt for the session, or $/cancel_request can cancel. storeTurn
// stores the turn as the prompt-overlap policy has it, or refuses it, which
// is answered at once without the handler. As the handler returns, the turn
// is removed and its context's cause read once, under the lock session/cancel
// takes: a turn cancelled by then is answered with the cancelled stop reason,
// keeping the rest of the handler's result, and a cancel that comes later
// finds no turn and changes nothing
// (0017-MADR-remediate-acp-v1-conformance-violations.md, P1;
// 0021-PLAN-resolve-issues-4-and-5-before-v1-14-0.md, step 3).
func promptCase(f *goFile, recv ast.Expr, mi *model.Method, gate ast.Expr) []ast.Stmt {
	field := mi.SessionField
	refused := call(id("NewInternalError"), composite(rawMap(id("any")),
		kv(str("error"), str("a prompt turn is already running for this session")),
		kv(str(mi.SessionKey), dot("p", field))))
	return []ast.Stmt{
		define(exprs(id("r")), call(id("receiptOf"), id("ctx"))),
		define(exprs(id("reqCtx"), id("cancel")), call(f.qual(pkgContext, "WithCancelCause"), id("ctx"))),
		define(exprs(id("turn")), addr(composite(id("promptTurn"), kv(id("cancel"), id("cancel")), kv(id("receipt"), id("r"))))),
		lockCall("Lock"),
		define(exprs(id("stored")), call(dot("a", "storeTurn"), sessionKey(field), id("turn"))),
		lockCall("Unlock"),
		ifStmt(nil, not(id("stored")),
			exprStmt(call(id("cancel"), nilExpr())),
			ret(nilExpr(), refused)),
		define(exprs(id("resp"), id("err")), call(sel(recv, mi.GoName), id("reqCtx"), id("p"))),
		lockCall("Lock"),
		exprStmt(call(dot("a", "removeTurn"), sessionKey(field), id("turn"))),
		define(exprs(id("cause")), call(f.qual(pkgContext, "Cause"), id("reqCtx"))),
		lockCall("Unlock"),
		exprStmt(call(id("cancel"), nilExpr())),
		ifStmt(nil, binary(id("promptReturned"), token.NEQ, nilExpr()), exprStmt(call(id("promptReturned")))),
		ifStmt(nil, binary(id("cause"), token.NEQ, nilExpr()),
			ifStmt(nil, errNotNil(), assign(exprs(id("resp")), composite(id(mi.Resp)))),
			assign(exprs(dot("resp", mi.StopField), id("err")), id(mi.StopConst), nilExpr()),
		),
		ifStmt(nil, errNotNil(), toRequestError()),
		checkedResult(dot("a", "conn"), gate),
	}
}

// cancelActivePrompt cancels the in-flight prompt-turn context of the session,
// if any, with ErrPromptCancelled as its cause.
//
// A turn received after the cancel is not its: a cancel the receive loop read
// before the prompt does not apply to it. Every cancel is kept in lateCancels,
// whether or not it found a turn, for a prompt received before it whose turn
// is not stored yet (0017-PLAN-remediate-acp-v1-conformance-violations.md,
// step 5 deviation; 0019-PLAN-behaviour-fixes.md, B5); keepCancel bounds what
// is kept (B7). A cancel with no receipt applies to the running turn.
func cancelActivePrompt(field string) []ast.Stmt {
	return []ast.Stmt{
		define(exprs(id("s")), call(id("receiptOf"), id("ctx"))),
		lockCall("Lock"),
		exprStmt(call(dot("a", "cancelTurns"), sessionKey(field), id("s"))),
		exprStmt(call(dot("a", "conn", "keepCancel"), dot("a", "lateCancels"), sessionKey(field), id("s"))),
		lockCall("Unlock"),
	}
}

// side is what tells the two side connections apart.
type side struct {
	// typeName, ctor and local name the type, its constructor and the
	// constructor's variable; field and iface the handler it dispatches to;
	// aware and setter the hand-written interface through which the
	// constructor hands the handler its connection, and its method.
	typeName, ctor, local, field, iface, aware, setter string
	// recv is the receiver name of the type's generated methods.
	recv string
	// doc and ctorDoc are the type's and the constructor's doc comments; peer
	// names the other side in Connection's.
	doc, ctorDoc []string
	peer         string
	// promptTurns is true when the side holds prompt-turn contexts, pending
	// when it answers requests itself on a prompt-turn cancel, and negotiated
	// when it records what initialize agreed.
	promptTurns, pending, negotiated bool
}

// agentSide and clientSide describe the two side connections of m
// (0017-MADR-remediate-acp-v1-conformance-violations.md, S1).
func agentSide(m *model.Model) side {
	return side{
		typeName: "AgentSideConnection", ctor: "NewAgentSideConnection", local: "asc", field: "agent", recv: "a",
		iface: m.InterfaceOf(model.BindAgent).Name, aware: "AgentConnAware", setter: "SetAgentConnection",
		doc: []string{"AgentSideConnection represents the agent's view of a connection to a client."},
		ctorDoc: []string{
			"NewAgentSideConnection creates a new agent-side connection bound to the",
			"provided Agent implementation. Optional ConnectionOption values configure",
			"the underlying connection (e.g., WithMaxQueuedNotifications).",
		},
		peer:       "client",
		negotiated: m.Initialize != nil,
		promptTurns: slices.ContainsFunc(m.AgentMethods, func(mi *model.Method) bool {
			return mi.PromptTurn || mi.CancelsPromptTurn
		}),
	}
}

func clientSide(m *model.Model) side {
	return side{
		typeName: "ClientSideConnection", ctor: "NewClientSideConnection", local: "csc", field: "client", recv: "c",
		iface: m.InterfaceOf(model.BindClient).Name, aware: "ClientConnAware", setter: "SetClientConnection",
		doc: []string{"ClientSideConnection provides the client's view of the connection and implements Agent calls."},
		ctorDoc: []string{
			"NewClientSideConnection creates a new client-side connection bound to the",
			"provided Client implementation. Optional ConnectionOption values configure",
			"the underlying connection (e.g., WithMaxQueuedNotifications).",
		},
		peer:       "agent",
		negotiated: m.Initialize != nil,
		pending: slices.ContainsFunc(m.ClientMethods, func(mi *model.Method) bool {
			return mi.CancelledByPromptCancel
		}),
	}
}

// emitSideConnection emits a side connection: its struct, which holds the
// state its rules need, its constructor, and the methods that forward to its
// Connection.
func emitSideConnection(f *goFile, s side) {
	fs := []*ast.Field{param("conn", star(id("Connection"))), param(s.field, id(s.iface))}
	if s.promptTurns {
		fs = append(fs,
			param("mu", f.qual(pkgSync, "Mutex")),
			param("sessionCancels", mapOf(id("string"), sliceOf(star(id("promptTurn"))))),
			param("lateCancels", mapOf(id("string"), id("uint64"))))
	}
	if s.pending {
		fs = append(fs, param("pending", id("pendingRequests")))
	}
	if s.negotiated {
		fs = append(fs, param("negotiation", index(f.qual(pkgAtomic, "Pointer"), id("Negotiation"))))
	}
	f.comment(s.doc...)
	f.add(typeDecl(s.typeName, &ast.StructType{Fields: fields(fs...)}))

	body := []ast.Stmt{
		define(exprs(id(s.local)), addr(composite(id(s.typeName)))),
		assign(exprs(dot(s.local, s.field)), id(s.field)),
	}
	if s.promptTurns {
		body = append(body, assign(exprs(dot(s.local, "sessionCancels")),
			call(id("make"), mapOf(id("string"), sliceOf(star(id("promptTurn")))))),
			assign(exprs(dot(s.local, "lateCancels")), call(id("make"), mapOf(id("string"), id("uint64")))))
	}
	// The connection is assigned before it starts reading, so a message that
	// is already waiting never reaches handle with conn unset
	// (0018-MADR-start-side-connections-after-assignment.md); the handler is
	// handed it in between, for the same reason
	// (0019-PLAN-api-decisions.md, B2).
	body = append(body,
		assign(exprs(dot(s.local, "conn")), &ast.CallExpr{
			Fun:      id("buildConnection"),
			Args:     exprs(dot(s.local, "handleWithExtensions"), id("peerInput"), id("peerOutput"), id("opts")),
			Ellipsis: placeholderPos,
		}),
		ifStmt(define(exprs(id("aware"), id("ok")), typeAssert(id(s.field), id(s.aware))), id("ok"),
			exprStmt(call(dot("aware", s.setter), id(s.local)))),
		exprStmt(call(dot(s.local, "conn", "start"))),
		ret(id(s.local)),
	)
	f.line()
	f.comment(s.ctorDoc...)
	f.add(function(s.ctor, funcType([]*ast.Field{
		param(s.field, id(s.iface)),
		param("peerInput", f.qual(pkgIO, "Writer")),
		param("peerOutput", f.qual(pkgIO, "Reader")),
		param("opts", &ast.Ellipsis{Elt: id("ConnectionOption")}),
	}, param("", star(id(s.typeName)))), body...))

	recv := func() *ast.Field { return param(s.recv, star(id(s.typeName))) }
	f.line()
	f.comment("DroppedNotifications reports how many inbound notifications the underlying",
		"connection has dropped under OverflowDropNewest.")
	f.add(method(recv(), "DroppedNotifications", funcType(nil, param("", id("uint64"))),
		ret(call(dot(s.recv, "conn", "DroppedNotifications")))))
	f.line()
	f.comment("Done exposes a channel that closes when the connection ends: the peer",
		"disconnects, or Close or the overflow policy closes it.")
	f.add(method(recv(), "Done", funcType(nil, param("", &ast.ChanType{Dir: ast.RECV, Value: &ast.StructType{Fields: fields()}})),
		ret(call(dot(s.recv, "conn", "Done")))))
	f.line()
	f.comment("Close closes the underlying connection; see Connection.Close",
		"(0019-PLAN-api-decisions.md, B17).")
	f.add(method(recv(), "Close", funcType(nil, param("", id("error"))),
		ret(call(dot(s.recv, "conn", "Close")))))
	f.line()
	f.comment("SetLogger directs connection diagnostics to the provided logger.")
	f.add(method(recv(), "SetLogger", funcType([]*ast.Field{param("l", star(f.qual(pkgSlog, "Logger")))}),
		exprStmt(call(dot(s.recv, "conn", "SetLogger"), id("l")))))
	f.line()
	f.comment(
		"Connection returns the underlying JSON-RPC connection.",
		"",
		"It is for requests the typed methods cannot make: a method the "+s.peer,
		"implements that this SDK's schema does not carry, or one whose response has",
		"fields the generated type drops. CallExtension covers only \"_\"-prefixed",
		"methods; for anything else, use SendRequest or SendNotification on the",
		"returned connection. It is the same connection the typed methods use, not a",
		"copy. It holds no record of initialize, so what is sent through it is not",
		"checked against the negotiated capabilities.",
	)
	f.add(method(recv(), "Connection", funcType(nil, param("", star(id("Connection")))), ret(dot(s.recv, "conn"))))
	f.line()
	// The hand-written extension dispatch, compiled with every schema, takes
	// its gate from here (0019-PLAN-api-decisions.md, B14).
	var inbound ast.Expr = nilExpr()
	if s.negotiated {
		inbound = dot(s.recv, "gate")
	}
	f.comment("inboundGate is the gate of the received messages the generated dispatch",
		"does not decode, the extension messages: gate, or nil when the schema has",
		"no initialize (0019-PLAN-api-decisions.md, B14).")
	f.add(method(recv(), "inboundGate", funcType(nil, param("", id("gateFunc"))), ret(inbound)))
	f.line()
	if s.negotiated {
		f.comment("gate checks a message of this connection against what initialize agreed",
			"(0017-MADR-remediate-acp-v1-conformance-violations.md, G1).")
		f.add(method(recv(), "gate", funcType([]*ast.Field{param("method", id("string")), param("params", id("any"))}, param("", id("error"))),
			ret(call(id("gateMessage"), call(dot(s.recv, "negotiation", "Load")), id("method"), id("params")))))
		f.line()
		f.comment("resultGate checks a result of this connection, sent or received, against",
			"what initialize agreed.")
		f.add(method(recv(), "resultGate", funcType([]*ast.Field{param("method", id("string")), param("result", id("any"))}, param("", id("error"))),
			ret(call(id("gateResult"), call(dot(s.recv, "negotiation", "Load")), id("method"), id("result")))))
		f.line()
		f.comment("Negotiated returns what initialize agreed on this connection, the request",
			"and the response of its last successful initialize, and false before one",
			"(0017-MADR-remediate-acp-v1-conformance-violations.md, G1). The record is",
			"the JSON form of what was sent and received, and each call returns a copy",
			"of it that shares no memory with the connection's or the caller's values",
			"(0019-PLAN-behaviour-fixes.md, B13).")
		f.add(method(recv(), "Negotiated", funcType(nil, param("", id("Negotiation")), param("", id("bool"))),
			define(exprs(id("n")), call(dot(s.recv, "negotiation", "Load"))),
			ifStmt(nil, binary(id("n"), token.EQL, nilExpr()), ret(composite(id("Negotiation")), id("false"))),
			define(exprs(id("rec"), id("err")), call(id("jsonCopy"), star(id("n")))),
			ifStmt(nil, errNotNil(), ret(composite(id("Negotiation")), id("false"))),
			ret(id("rec"), id("true")),
		))
		f.line()
	}
	if s.promptTurns {
		f.comment("promptTurn is one prompt turn: its cancel function and the receipt number",
			"of its request. sessionCancels holds the running turns of each session by",
			"pointer, so a turn removes only its own entry, and lateCancels the receipt of",
			"each session's latest cancel, for a prompt received before it and not stored",
			"yet (0017-MADR-remediate-acp-v1-conformance-violations.md, P1;",
			"0021-PLAN-resolve-issues-4-and-5-before-v1-14-0.md, step 3).")
		f.add(typeDecl("promptTurn", &ast.StructType{Fields: fields(
			param("cancel", f.qual(pkgContext, "CancelCauseFunc")), param("receipt", id("uint64")))}))
		f.line()
		f.comment("promptReturned, when a test sets it, runs after a prompt turn's handler",
			"returns and the turn is removed, before its answer is decided. It is nil",
			"outside tests.")
		f.add(&ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
			Names: []*ast.Ident{id("promptReturned")}, Type: &ast.FuncType{Params: fields()}}}})
		f.line()
		f.comment("errSuperseded is the cause of a prompt turn that a newer prompt of its",
			"session superseded (0019-PLAN-idioms-performance.md, D7).")
		f.add(&ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
			Names:  []*ast.Ident{id("errSuperseded")},
			Values: exprs(call(f.qual(pkgFmt, "Errorf"), str("%w: superseded by a new prompt"), id("ErrPromptCancelled"))),
		}}})
		f.line()
		emitTurnStore(f, s)
	}
}

// emitTurnStore emits the side's prompt-turn store: storeTurn, removeTurn
// and cancelTurns, each called under a.mu, which keep sessionCancels by the
// connection's prompt-overlap policy
// (0021-PLAN-resolve-issues-4-and-5-before-v1-14-0.md, step 3).
func emitTurnStore(f *goFile, s side) {
	recv := func() *ast.Field { return param("a", star(id(s.typeName))) }
	turns := func() ast.Expr { return index(dot("a", "sessionCancels"), id("key")) }
	single := func() ast.Expr { return composite(sliceOf(star(id("promptTurn"))), id("turn")) }
	keyTurn := func() []*ast.Field {
		return []*ast.Field{param("key", id("string")), param("turn", star(id("promptTurn")))}
	}
	late := binary(binary(dot("turn", "receipt"), token.NEQ, intLit(0)), token.LAND,
		binary(index(dot("a", "lateCancels"), id("key")), token.GTR, dot("turn", "receipt")))
	f.comment("storeTurn stores turn, a prompt turn of the session key, as the connection's",
		"prompt-overlap policy has it, and reports false when the policy refuses it.",
		"Supersede, the default, cancels the stored turns, or this one when a stored",
		"turn was received after it; refuse refuses it while a turn is stored;",
		"concurrent adds it. A kept cancel received after the prompt cancels it, and",
		"under supersede a prompt newer than the kept cancel ends it, a prompt stored",
		"after a newer one being superseded anyway (0019-PLAN-behaviour-fixes.md, B5).",
		"The caller holds a.mu.")
	f.add(method(recv(), "storeTurn", funcType(keyTurn(), param("", id("bool"))),
		define(exprs(id("supersedes")), boolLit(false)),
		switchStmt(dot("a", "conn", "promptOverlap"),
			caseOf(exprs(id("PromptOverlapRefuse")),
				ifStmt(nil, binary(call(id("len"), turns()), token.GTR, intLit(0)), ret(boolLit(false))),
				assign(exprs(turns()), single())),
			caseOf(exprs(id("PromptOverlapConcurrent")),
				assign(exprs(turns()), call(id("append"), turns(), id("turn")))),
			caseOf(nil,
				rangeStmt(id("_"), id("prev"), turns(),
					ifStmt(nil, binary(dot("prev", "receipt"), token.GTR, dot("turn", "receipt")),
						exprStmt(call(dot("turn", "cancel"), id("errSuperseded"))), ret(boolLit(true)))),
				rangeStmt(id("_"), id("prev"), turns(), exprStmt(call(dot("prev", "cancel"), id("errSuperseded")))),
				assign(exprs(turns()), single()),
				assign(exprs(id("supersedes")), boolLit(true))),
		),
		elseIf(ifStmt(nil, late, exprStmt(call(dot("turn", "cancel"), id("ErrPromptCancelled")))),
			ifStmt(nil, id("supersedes"), exprStmt(call(id("delete"), dot("a", "lateCancels"), id("key"))))),
		ret(boolLit(true)),
	))
	f.line()
	keep := func(body ast.Stmt) []ast.Stmt {
		return []ast.Stmt{
			varStmt("kept", sliceOf(star(id("promptTurn")))),
			rangeStmt(id("_"), id("t"), turns(), body),
			ifElse(nil, binary(call(id("len"), id("kept")), token.EQL, intLit(0)),
				[]ast.Stmt{exprStmt(call(id("delete"), dot("a", "sessionCancels"), id("key")))},
				[]ast.Stmt{assign(exprs(turns()), id("kept"))}),
		}
	}
	appendKept := assign(exprs(id("kept")), call(id("append"), id("kept"), id("t")))
	f.comment("removeTurn removes turn from the turns of the session key. The caller holds a.mu.")
	f.add(method(recv(), "removeTurn", funcType(keyTurn()),
		keep(ifStmt(nil, binary(id("t"), token.NEQ, id("turn")), appendKept))...))
	f.line()
	applies := binary(binary(id("s"), token.EQL, intLit(0)), token.LOR, binary(dot("t", "receipt"), token.LSS, id("s")))
	f.comment("cancelTurns cancels and removes every turn of the session key received before",
		"the cancel whose receipt is s, or every one when s is 0. The caller holds a.mu.")
	f.add(method(recv(), "cancelTurns", funcType([]*ast.Field{param("key", id("string")), param("s", id("uint64"))}),
		keep(ifElse(nil, applies,
			[]ast.Stmt{exprStmt(call(dot("t", "cancel"), id("ErrPromptCancelled")))},
			[]ast.Stmt{assign(exprs(id("kept")), call(id("append"), id("kept"), id("t")))}))...))
	f.line()
}

// gateOf returns, for each use, a new expression of the side's gate called
// name: recv.<name> when the model has an initialize rule, nil otherwise.
func gateOf(m *model.Model, recv, name string) func() ast.Expr {
	return func() ast.Expr {
		if m.Initialize == nil {
			return nilExpr()
		}
		return dot(recv, name)
	}
}

// capabilityMissing is the expression over n, a Negotiation, that is true when
// a capability is not advertised: root is the side's capabilities in n, and
// steps the fields of the path from it. A pointer on the path is missing when
// nil, and a final bool when false.
func capabilityMissing(root func() ast.Expr, steps []model.CapabilityStep) ast.Expr {
	upTo := func(k int) ast.Expr {
		x := root()
		for _, st := range steps[:k+1] {
			x = sel(x, st.Field)
		}
		return x
	}
	var cond ast.Expr
	or := func(e ast.Expr) {
		if cond == nil {
			cond = e
			return
		}
		cond = binary(cond, token.LOR, e)
	}
	for i, st := range steps {
		switch {
		case st.Pointer:
			or(binary(upTo(i), token.EQL, nilExpr()))
		case i == len(steps)-1:
			or(not(upTo(i)))
		}
	}
	return cond
}

// emitGates emits Negotiation.Allows, the method gates of the protocol table,
// and gateMessage, which every typed message goes through
// (0017-MADR-remediate-acp-v1-conformance-violations.md, G1). Methods gated by
// one capability share a case.
func emitGates(f *goFile, m *model.Model) {
	type gate struct {
		root    func() ast.Expr
		mi      *model.Method
		methods []ast.Expr
	}
	var gates []*gate
	byCapability := map[string]*gate{}
	collect := func(methods []*model.Method, root func() ast.Expr) {
		for _, mi := range methods {
			if mi.Capability == "" {
				continue
			}
			key := mi.Side + " " + mi.Capability
			g := byCapability[key]
			if g == nil {
				g = &gate{root: root, mi: mi}
				byCapability[key] = g
				gates = append(gates, g)
			}
			g.methods = append(g.methods, id(mi.Const))
		}
	}
	collect(m.AgentMethods, func() ast.Expr { return dot("n", "Response", m.Initialize.RespCapsField) })
	collect(m.ClientMethods, func() ast.Expr { return dot("n", "Request", m.Initialize.ReqCapsField) })
	var cases []ast.Stmt
	for _, g := range gates {
		cases = append(cases, caseOf(g.methods,
			ifStmt(nil, capabilityMissing(g.root, g.mi.CapabilitySteps),
				ret(addr(composite(id("CapabilityError"), kv(id("Method"), id("method")), kv(id("Capability"), str(g.mi.Capability))))))))
	}
	body := []ast.Stmt{}
	if len(cases) > 0 {
		body = append(body, switchStmt(id("method"), cases...))
	}
	body = append(body, ret(nilExpr()))
	f.line()
	f.comment("Allows reports whether the capabilities n records let method be sent: nil,",
		"or a *CapabilityError naming the capability that is not advertised",
		"(0017-MADR-remediate-acp-v1-conformance-violations.md, G1).")
	f.add(method(param("n", id("Negotiation")), "Allows", funcType([]*ast.Field{param("method", id("string"))}, param("", id("error"))), body...))
	emitGater(f)
	f.line()
	f.comment("gateMessage checks a typed message against the record n of initialize:",
		"initialize itself passes; before it, nothing does; after it, a method the",
		"record does not allow does not, nor a property or variant it does not.")
	f.add(function("gateMessage", funcType([]*ast.Field{
		param("n", star(id("Negotiation"))), param("method", id("string")), param("params", id("any")),
	}, param("", id("error"))),
		ifStmt(nil, binary(id("method"), token.EQL, id(m.Initialize.Const)), ret(nilExpr())),
		ifStmt(nil, binary(id("n"), token.EQL, nilExpr()),
			ret(call(f.qual(pkgFmt, "Errorf"), str("%s: %w"), id("method"), id("ErrNotInitialized")))),
		ifStmt(define(exprs(id("err")), call(dot("n", "Allows"), id("method"))), errNotNil(), ret(id("err"))),
		ifStmt(define(exprs(id("g"), id("ok")), typeAssert(id("params"), id("gater"))), id("ok"),
			ret(call(dot("g", "gates"), id("n"), id("method")))),
		ret(nilExpr()),
	))
	f.line()
	f.comment("gateResult checks a result of method against the property and variant",
		"gates of the record n; the method itself was checked when the request was",
		"sent or received (0017-MADR-remediate-acp-v1-conformance-violations.md, G1).")
	f.add(function("gateResult", funcType([]*ast.Field{
		param("n", star(id("Negotiation"))), param("method", id("string")), param("result", id("any")),
	}, param("", id("error"))),
		ifStmt(define(exprs(id("g"), id("ok")), typeAssert(id("result"), id("gater"))), binary(id("ok"), token.LAND, binary(id("n"), token.NEQ, nilExpr())),
			ret(call(dot("g", "gates"), id("n"), id("method")))),
		ret(nilExpr()),
	))
	emitValueGates(f, m)
}

// agentFile builds agent_gen.go: the agent's handler and its outbound wrappers.
func agentFile(m *model.Model) *goFile {
	f := newFile()
	emitSideConnection(f, agentSide(m))
	gate, resultGate := gateOf(m, "a", "gate"), gateOf(m, "a", "resultGate")

	var cases []ast.Stmt
	for _, mi := range m.AgentMethods {
		var body []ast.Stmt
		if mi.Notif != "" {
			body = append(body, kindCheck(true))
			body = append(body, unmarshalValidate(dot("a", "conn"), mi.Notif, gate())...)
			// A prompt turn's cancel first cancels the session's in-flight prompt context.
			if mi.CancelsPromptTurn {
				body = append(body, cancelActivePrompt(mi.SessionField)...)
			}
			pre, recv := agentAssert(f, m.InterfaceOf(mi.Binding), mi.GoName, mi.Notif, "", false)
			body = append(body, pre...)
			body = append(body, callNoResult(recv, mi.GoName)...)
		} else {
			body = append(body, kindCheck(false))
			body = append(body, unmarshalValidate(dot("a", "conn"), mi.Req, gate())...)
			pre, recv := agentAssert(f, m.InterfaceOf(mi.Binding), mi.GoName, mi.Req, mi.Resp, mi.HasResult)
			body = append(body, pre...)
			// A session close first does what the prompt-turn cancel does
			// (0019-PLAN-behaviour-fixes.md, B4).
			if mi.ClosesSession {
				body = append(body, cancelActivePrompt(mi.SessionField)...)
			}
			switch {
			case mi.Initializes:
				body = append(body, initializeCase(recv, mi)...)
			case mi.PromptTurn:
				body = append(body, promptCase(f, recv, mi, resultGate())...)
			case !mi.HasResult:
				body = append(body, callNoResult(recv, mi.GoName)...)
			default:
				body = append(body, callWithResult(dot("a", "conn"), recv, mi.GoName, resultGate())...)
			}
		}
		cases = append(cases, caseOf(exprs(id(mi.Const)), body...))
	}
	f.add(handleFunc(f, "a", "AgentSideConnection", cases))

	// Agent outbound wrappers (agent -> client).
	emitOutbound(f, agentSide(m), m.ClientMethods, m.Initialize != nil)

	return f
}

// clientFile builds client_gen.go: the client's handler and its outbound wrappers.
func clientFile(m *model.Model) *goFile {
	f := newFile()
	emitSideConnection(f, clientSide(m))
	gate, resultGate := gateOf(m, "c", "gate"), gateOf(m, "c", "resultGate")

	var cases []ast.Stmt
	for _, mi := range m.ClientMethods {
		var body []ast.Stmt
		if mi.Notif != "" {
			body = append(body, kindCheck(true))
			body = append(body, unmarshalValidate(dot("c", "conn"), mi.Notif, gate())...)
			pre, recv := clientAssert(f, m.InterfaceOf(mi.Binding), mi.GoName, mi.Notif, "", false)
			body = append(body, pre...)
			body = append(body, callNoResult(recv, mi.GoName)...)
		} else {
			body = append(body, kindCheck(false))
			body = append(body, unmarshalValidate(dot("c", "conn"), mi.Req, gate())...)
			pre, recv := clientAssert(f, m.InterfaceOf(mi.Binding), mi.GoName, mi.Req, mi.Resp, mi.HasResult)
			body = append(body, pre...)
			switch {
			case mi.CancelledByPromptCancel:
				body = append(body, cancelledOnPromptCancel(f, recv, mi, resultGate())...)
			case mi.HasResult:
				body = append(body, callWithResult(dot("c", "conn"), recv, mi.GoName, resultGate())...)
			default:
				body = append(body, callNoResult(recv, mi.GoName)...)
			}
		}
		cases = append(cases, caseOf(exprs(id(mi.Const)), body...))
	}
	f.add(handleFunc(f, "c", "ClientSideConnection", cases))

	// Client outbound wrappers (client -> agent).
	emitOutbound(f, clientSide(m), m.AgentMethods, m.Initialize != nil)

	return f
}

// cancelledOnPromptCancel calls the method under a context its session's
// prompt-turn cancel ends, and answers with the cancelled variant at once
// when it does (0017-MADR-remediate-acp-v1-conformance-violations.md, Q1).
func cancelledOnPromptCancel(f *goFile, recv ast.Expr, mi *model.Method, gate ast.Expr) []ast.Stmt {
	callFn := &ast.FuncLit{
		Type: funcType([]*ast.Field{param("ctx", f.qual(pkgContext, "Context"))}, param("", id(mi.Resp)), param("", id("error"))),
		Body: block(ret(recvCall(recv, mi.GoName))),
	}
	answer := composite(id(mi.Resp), kv(id(mi.CancelledField), call(id(mi.CancelledHelper))))
	return []ast.Stmt{
		define(exprs(id("pctx"), id("done")), call(dot("c", "pending", "track"), id("ctx"), sessionKey(mi.SessionField))),
		&ast.DeferStmt{Call: call(id("done"))},
		define(exprs(id("resp"), id("err")), call(id("answerUnlessCancelled"), dot("c", "conn"), id("pctx"), id("method"), callFn, answer)),
		ifStmt(nil, errNotNil(), toRequestError()),
		checkedResult(dot("c", "conn"), gate),
	}
}

// initializeCase answers initialize: a version the handler leaves zero is
// ProtocolVersionNumber, and any other is refused with -32603, as this SDK
// speaks that version only; the record of the request and the answer is
// stored before the answer is written; the answer's gates are checked
// against that record, there being none before it
// (0017-MADR-remediate-acp-v1-conformance-violations.md, N1 and G1).
func initializeCase(recv ast.Expr, mi *model.Method) []ast.Stmt {
	gate := gateFuncLit(ret(call(id("gateResult"),
		addr(composite(id("Negotiation"), kv(id("Request"), id("p")), kv(id("Response"), id("resp")))), id("method"), id("result"))))
	version := dot("resp", mi.RespVersionField)
	verr := addr(composite(id("ProtocolVersionError"),
		kv(id("Requested"), call(id("int"), dot("p", mi.ReqVersionField))),
		kv(id("Answered"), call(id("int"), dot("resp", mi.RespVersionField)))))
	return []ast.Stmt{
		define(exprs(id("resp"), id("err")), recvCall(recv, mi.GoName)),
		ifStmt(nil, errNotNil(), toRequestError()),
		ifStmt(nil, binary(version, token.EQL, intLit(0)), assign(exprs(dot("resp", mi.RespVersionField)), id("ProtocolVersionNumber"))),
		ifStmt(nil, binary(dot("resp", mi.RespVersionField), token.NEQ, id("ProtocolVersionNumber")),
			define(exprs(id("verr")), verr),
			exprStmt(call(sel(call(dot("a", "conn", "loggerOrDefault")), "Error"), str("initialize answer not sent"), str("method"), id("method"), str("err"), id("verr"))),
			ret(nilExpr(), call(id("NewInternalError"), composite(rawMap(id("any")), kv(str("error"), call(dot("verr", "Error")))))),
		),
		define(exprs(id("result"), id("rerr")), call(dot("a", "conn", "checkResult"), id("method"), id("resp"), gate)),
		ifStmt(nil, binary(id("rerr"), token.NEQ, nilExpr()), ret(nilExpr(), id("rerr"))),
		// The record is the JSON form of the request and the answer, sharing
		// no memory with the handler's values (0019-PLAN-behaviour-fixes.md, B13).
		define(exprs(id("rec"), id("cerr")), call(id("jsonCopy"),
			composite(id("Negotiation"), kv(id("Request"), id("p")), kv(id("Response"), id("resp"))))),
		ifStmt(nil, binary(id("cerr"), token.NEQ, nilExpr()),
			exprStmt(call(sel(call(dot("a", "conn", "loggerOrDefault")), "Error"), str("initialize answer not sent"), str("method"), id("method"), str("err"), id("cerr"))),
			ret(nilExpr(), call(id("NewInternalError"), composite(rawMap(id("any")), kv(str("error"), call(dot("cerr", "Error")))))),
		),
		exprStmt(call(dot("a", "negotiation", "Store"), addr(id("rec")))),
		ret(id("result"), nilExpr()),
	}
}

// initializeWrapper sends initialize: a version the request leaves zero is
// ProtocolVersionNumber, another is refused before sending, and an answer
// with another version is a *ProtocolVersionError and records nothing; the
// answer's gates are checked against the record it would make, through the
// validation hook as any response is
// (0017-MADR-remediate-acp-v1-conformance-violations.md, N1 and G1).
func initializeWrapper(mi *model.Method) []ast.Stmt {
	req, resp := dot("params", mi.ReqVersionField), dot("resp", mi.RespVersionField)
	gate := gateFuncLit(ret(call(id("gateResult"),
		addr(composite(id("Negotiation"), kv(id("Request"), id("params")), kv(id("Response"), star(typeAssert(id("result"), star(id(mi.Resp))))))),
		id("method"), id("result"))))
	return []ast.Stmt{
		ifStmt(nil, binary(req, token.EQL, intLit(0)), assign(exprs(dot("params", mi.ReqVersionField)), id("ProtocolVersionNumber"))),
		ifStmt(nil, binary(dot("params", mi.ReqVersionField), token.NEQ, id("ProtocolVersionNumber")),
			ret(composite(id(mi.Resp)), addr(composite(id("ProtocolVersionError"),
				kv(id("Requested"), call(id("int"), dot("params", mi.ReqVersionField))))))),
		define(exprs(id("resp"), id("err")),
			call(index(id("sendRequest"), id(mi.Resp)), dot("c", "conn"), id("ctx"), id(mi.Const), id("params"), gate)),
		ifStmt(nil, errNotNil(), ret(id("resp"), id("err"))),
		ifStmt(nil, binary(resp, token.NEQ, id("ProtocolVersionNumber")),
			ret(id("resp"), addr(composite(id("ProtocolVersionError"),
				kv(id("Requested"), call(id("int"), dot("params", mi.ReqVersionField))),
				kv(id("Answered"), call(id("int"), dot("resp", mi.RespVersionField))))))),
		// The record is the JSON form of what was sent and received, sharing no
		// memory with the caller's values (0019-PLAN-behaviour-fixes.md, B13).
		define(exprs(id("rec"), id("err")), call(id("jsonCopy"),
			composite(id("Negotiation"), kv(id("Request"), id("params")), kv(id("Response"), id("resp"))))),
		ifStmt(nil, errNotNil(), ret(id("resp"), id("err"))),
		exprStmt(call(dot("c", "negotiation", "Store"), addr(id("rec")))),
		ret(id("resp"), nilExpr()),
	}
}

// outboundDoc is a typed outbound method's doc: what it sends to peer, then
// the interface method's doc (0019-PLAN-examples-docs-tests-tooling.md, I12).
func outboundDoc(mi *model.Method, peer string) []string {
	first := mi.GoName + " sends the " + mi.Wire + " request to the " + peer + " and returns its response."
	switch {
	case mi.Notif != "":
		first = mi.GoName + " sends the " + mi.Wire + " notification to the " + peer + "."
	case !mi.HasResult:
		first = mi.GoName + " sends the " + mi.Wire + " request to the " + peer + " and returns the error it is answered with, or nil."
	}
	if len(mi.Doc) == 0 {
		return []string{first}
	}
	return append([]string{first, ""}, mi.Doc...)
}

// emitOutbound emits one method per peer method on recvType that sends the
// request or notification over c.conn to peer.
func emitOutbound(f *goFile, s side, methods []*model.Method, gated bool) {
	recv := func() *ast.Field { return param(s.recv, star(id(s.typeName))) }
	params := func(typeName string) []*ast.Field {
		return []*ast.Field{param("ctx", f.qual(pkgContext, "Context")), param("params", id(typeName))}
	}
	// gateFirst is if err := c.gate(<const>, &params); err != nil { return [zero,] err }:
	// a message the record does not allow is not written
	// (0017-MADR-remediate-acp-v1-conformance-violations.md, G1).
	gateFirst := func(mi *model.Method, zero ast.Expr) []ast.Stmt {
		if !gated {
			return nil
		}
		fail := ret(id("err"))
		if zero != nil {
			fail = ret(zero, id("err"))
		}
		return []ast.Stmt{ifStmt(define(exprs(id("err")), call(dot(s.recv, "gate"), id(mi.Const), addr(id("params")))), errNotNil(), fail)}
	}
	// resultArg is the gate the response is checked by.
	resultArg := func() ast.Expr {
		if gated {
			return dot(s.recv, "resultGate")
		}
		return nilExpr()
	}

	for _, mi := range methods {
		// A blank line separates each wrapper from the declaration before it.
		f.line()
		f.comment(outboundDoc(mi, s.peer)...)
		if mi.Notif != "" && mi.CancelsPending {
			// The cancel is written before the pending requests are answered
			// (0017-MADR-remediate-acp-v1-conformance-violations.md, Q1).
			f.add(method(recv(), mi.GoName, funcType(params(mi.Notif), param("", id("error"))),
				append(gateFirst(mi, nil),
					define(exprs(id("err")), call(dot(s.recv, "conn", "SendNotification"), id("ctx"), id(mi.Const), id("params"))),
					exprStmt(call(dot(s.recv, "pending", "cancel"), call(id("string"), dot("params", mi.SessionField)))),
					ret(id("err")),
				)...,
			))
			continue
		}
		if mi.Notif != "" {
			f.add(method(recv(), mi.GoName, funcType(params(mi.Notif), param("", id("error"))),
				append(gateFirst(mi, nil), ret(call(dot(s.recv, "conn", "SendNotification"), id("ctx"), id(mi.Const), id("params"))))...,
			))
			continue
		}
		if !mi.HasResult {
			f.add(method(recv(), mi.GoName, funcType(params(mi.Req), param("", id("error"))),
				append(gateFirst(mi, nil), ret(call(dot(s.recv, "conn", "SendRequestNoResult"), id("ctx"), id(mi.Const), id("params"))))...,
			))
			continue
		}
		if mi.Initializes {
			f.add(method(recv(), mi.GoName, funcType(params(mi.Req), param("", id(mi.Resp)), param("", id("error"))),
				initializeWrapper(mi)...))
			continue
		}
		// A prompt turn learns whether its request was handed to the writer,
		// so it cancels only a turn the peer may have received
		// (0019-PLAN-issue-fixes.md, B22).
		canceller := mi.Canceller
		if !mi.PromptTurn {
			canceller = nil
		}
		send, results := "sendRequest", exprs(id("resp"), id("err"))
		if canceller != nil {
			send, results = "sendRequestSent", exprs(id("resp"), id("sent"), id("err"))
		}
		sendCall := call(index(id(send), id(mi.Resp)), dot(s.recv, "conn"), id("ctx"), id(mi.Const), id("params"), resultArg())
		sig := funcType(params(mi.Req), param("", id(mi.Resp)), param("", id("error")))
		// A wrapper with nothing to do after the call returns it.
		if canceller == nil && !mi.CancelsPending {
			f.add(method(recv(), mi.GoName, sig, append(gateFirst(mi, composite(id(mi.Resp))), ret(sendCall))...))
			continue
		}
		body := append(gateFirst(mi, composite(id(mi.Resp))), define(results, sendCall))
		// A session close the agent accepted has ended the session's turn, so
		// its pending requests are answered as on the prompt-turn cancel
		// (0019-PLAN-behaviour-fixes.md, B4).
		if mi.CancelsPending {
			body = append(body, ifStmt(nil, binary(id("err"), token.EQL, nilExpr()),
				exprStmt(call(dot(s.recv, "pending", "cancel"), call(id("string"), dot("params", mi.SessionField))))))
		}
		// A prompt turn whose context ended after its request was handed to
		// the writer sends its cancel, best effort, under a context that keeps
		// the caller's values without its end (I10). A cancel that fails is
		// logged at Debug, as the connection logs other best-effort work that
		// fails (0019-PLAN-idioms-performance.md, deviation during step 3).
		if cn := canceller; cn != nil {
			body = append(body, ifStmt(nil, errNotNil(),
				ifStmt(nil, binary(id("sent"), token.LAND, binary(call(dot("ctx", "Err")), token.NEQ, nilExpr())),
					ifStmt(define(exprs(id("cerr")), call(dot(s.recv, cn.GoName),
						call(f.qual(pkgContext, "WithoutCancel"), id("ctx")),
						composite(id(cn.Notif), kv(id(cn.SessionField), dot("params", mi.SessionField))),
					)), binary(id("cerr"), token.NEQ, nilExpr()),
						exprStmt(call(sel(call(dot(s.recv, "conn", "loggerOrDefault")), "Debug"),
							str("failed to send the cancel of a prompt turn"), str("method"), id(cn.Const), str("err"), id("cerr"))),
					),
				),
			))
		}
		body = append(body, ret(id("resp"), id("err")))
		f.add(method(recv(), mi.GoName, sig, body...))
	}
}
