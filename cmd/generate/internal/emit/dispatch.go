package emit

import (
	"go/ast"
	"go/token"
	"slices"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// sessionKey is string(p.<field>), the session's key in sessionCancels.
func sessionKey(field string) ast.Expr { return call(id("string"), dot("p", field)) }

// sessionCancel is a.sessionCancels[string(p.<field>)].
func sessionCancel(field string) ast.Expr {
	return index(dot("a", "sessionCancels"), sessionKey(field))
}

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
// later prompt for the session, or $/cancel_request can cancel. The turn is
// a pointer in sessionCancels, so a turn removes only its own entry. As the
// handler returns, the turn is removed and its context's cause read once,
// under the lock session/cancel takes: a turn cancelled by then is answered
// with the cancelled stop reason, keeping the rest of the handler's result,
// and a cancel that comes later finds no turn and changes nothing
// (0017-MADR-remediate-acp-v1-conformance-violations.md, P1).
func promptCase(f *goFile, recv ast.Expr, mi *model.Method, gate ast.Expr) []ast.Stmt {
	field := mi.SessionField
	return []ast.Stmt{
		define(exprs(id("r")), call(id("receiptOf"), id("ctx"))),
		define(exprs(id("reqCtx"), id("cancel")), call(f.qual(pkgContext, "WithCancelCause"), id("ctx"))),
		define(exprs(id("turn")), addr(composite(id("promptTurn"), kv(id("cancel"), id("cancel")), kv(id("receipt"), id("r"))))),
		define(exprs(id("superseded")), call(f.qual(pkgFmt, "Errorf"), str("%w: superseded by a new prompt"), id("ErrPromptCancelled"))),
		lockCall("Lock"),
		ifElse(define(exprs(id("prev"), id("ok")), sessionCancel(field)), binary(id("ok"), token.LAND, binary(dot("prev", "receipt"), token.GTR, id("r"))),
			[]ast.Stmt{exprStmt(call(id("cancel"), id("superseded")))},
			[]ast.Stmt{
				ifStmt(nil, id("ok"), exprStmt(call(dot("prev", "cancel"), id("superseded")))),
				assign(exprs(sessionCancel(field)), id("turn")),
				ifStmt(nil, binary(binary(id("r"), token.NEQ, intLit(0)), token.LAND,
					binary(index(dot("a", "lateCancels"), sessionKey(field)), token.GTR, id("r"))),
					exprStmt(call(id("cancel"), id("ErrPromptCancelled")))),
				exprStmt(call(id("delete"), dot("a", "lateCancels"), sessionKey(field))),
			},
		),
		lockCall("Unlock"),
		define(exprs(id("resp"), id("err")), call(sel(recv, mi.GoName), id("reqCtx"), id("p"))),
		lockCall("Lock"),
		ifStmt(nil, binary(sessionCancel(field), token.EQL, id("turn")),
			exprStmt(call(id("delete"), dot("a", "sessionCancels"), sessionKey(field)))),
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
// before the prompt does not apply to it. A cancel that finds no earlier turn
// is kept in lateCancels, for a prompt received before it whose turn is not
// stored yet (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 5
// deviation). A cancel with no receipt applies to the running turn.
func cancelActivePrompt(field string) []ast.Stmt {
	late := index(dot("a", "lateCancels"), sessionKey(field))
	applies := binary(binary(id("s"), token.EQL, intLit(0)), token.LOR, binary(dot("cn", "receipt"), token.LSS, id("s")))
	return []ast.Stmt{
		define(exprs(id("s")), call(id("receiptOf"), id("ctx"))),
		lockCall("Lock"),
		ifElse(define(exprs(id("cn"), id("ok")), sessionCancel(field)), binary(id("ok"), token.LAND, paren(applies)),
			[]ast.Stmt{
				exprStmt(call(dot("cn", "cancel"), id("ErrPromptCancelled"))),
				exprStmt(call(id("delete"), dot("a", "sessionCancels"), sessionKey(field))),
			},
			[]ast.Stmt{ifStmt(nil, binary(id("s"), token.GTR, late), assign(exprs(index(dot("a", "lateCancels"), sessionKey(field))), id("s")))},
		),
		lockCall("Unlock"),
	}
}

// side is what tells the two side connections apart.
type side struct {
	// typeName, ctor and local name the type, its constructor and the
	// constructor's variable; field and iface the handler it dispatches to.
	typeName, ctor, local, field, iface string
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
		typeName: "AgentSideConnection", ctor: "NewAgentSideConnection", local: "asc", field: "agent",
		iface: m.InterfaceOf(model.BindAgent).Name,
		doc:   []string{"AgentSideConnection represents the agent's view of a connection to a client."},
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
		typeName: "ClientSideConnection", ctor: "NewClientSideConnection", local: "csc", field: "client",
		iface: m.InterfaceOf(model.BindClient).Name,
		doc:   []string{"ClientSideConnection provides the client's view of the connection and implements Agent calls."},
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
			param("sessionCancels", mapOf(id("string"), star(id("promptTurn")))),
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
			call(id("make"), mapOf(id("string"), star(id("promptTurn"))))),
			assign(exprs(dot(s.local, "lateCancels")), call(id("make"), mapOf(id("string"), id("uint64")))))
	}
	body = append(body,
		assign(exprs(dot(s.local, "conn")), &ast.CallExpr{
			Fun:      id("NewConnection"),
			Args:     exprs(dot(s.local, "handleWithExtensions"), id("peerInput"), id("peerOutput"), id("opts")),
			Ellipsis: placeholderPos,
		}),
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

	recv := func() *ast.Field { return param("c", star(id(s.typeName))) }
	f.line()
	f.comment("DroppedNotifications reports how many inbound notifications the underlying",
		"connection has dropped under OverflowDropNewest.")
	f.add(method(recv(), "DroppedNotifications", funcType(nil, param("", id("uint64"))),
		ret(call(dot("c", "conn", "DroppedNotifications")))))
	f.line()
	f.comment("Done exposes a channel that closes when the peer disconnects.")
	f.add(method(recv(), "Done", funcType(nil, param("", &ast.ChanType{Dir: ast.RECV, Value: &ast.StructType{Fields: fields()}})),
		ret(call(dot("c", "conn", "Done")))))
	f.line()
	f.comment("SetLogger directs connection diagnostics to the provided logger.")
	f.add(method(recv(), "SetLogger", funcType([]*ast.Field{param("l", star(f.qual(pkgSlog, "Logger")))}),
		exprStmt(call(dot("c", "conn", "SetLogger"), id("l")))))
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
	f.add(method(recv(), "Connection", funcType(nil, param("", star(id("Connection")))), ret(dot("c", "conn"))))
	f.line()
	if s.negotiated {
		f.comment("gate checks a message of this connection against what initialize agreed",
			"(0017-MADR-remediate-acp-v1-conformance-violations.md, G1).")
		f.add(method(recv(), "gate", funcType([]*ast.Field{param("method", id("string")), param("params", id("any"))}, param("", id("error"))),
			ret(call(id("gateMessage"), call(dot("c", "negotiation", "Load")), id("method"), id("params")))))
		f.line()
		f.comment("resultGate checks a result of this connection, sent or received, against",
			"what initialize agreed.")
		f.add(method(recv(), "resultGate", funcType([]*ast.Field{param("method", id("string")), param("result", id("any"))}, param("", id("error"))),
			ret(call(id("gateResult"), call(dot("c", "negotiation", "Load")), id("method"), id("result")))))
		f.line()
		f.comment("Negotiated returns what initialize agreed on this connection, the request",
			"and the response of its last successful initialize, and false before one",
			"(0017-MADR-remediate-acp-v1-conformance-violations.md, G1).")
		f.add(method(recv(), "Negotiated", funcType(nil, param("", id("Negotiation")), param("", id("bool"))),
			define(exprs(id("n")), call(dot("c", "negotiation", "Load"))),
			ifStmt(nil, binary(id("n"), token.EQL, nilExpr()), ret(composite(id("Negotiation")), id("false"))),
			ret(star(id("n")), id("true")),
		))
		f.line()
	}
	if s.promptTurns {
		f.comment("promptTurn is one prompt turn: its cancel function and the receipt number",
			"of its request. sessionCancels holds the running turn of each session by",
			"pointer, so a turn removes only its own entry, and lateCancels the receipt of",
			"each session's latest cancel that found no earlier turn",
			"(0017-MADR-remediate-acp-v1-conformance-violations.md, P1).")
		f.add(typeDecl("promptTurn", &ast.StructType{Fields: fields(
			param("cancel", f.qual(pkgContext, "CancelCauseFunc")), param("receipt", id("uint64")))}))
		f.line()
		f.comment("promptReturned, when a test sets it, runs after a prompt turn's handler",
			"returns and the turn is removed, before its answer is decided. It is nil",
			"outside tests.")
		f.add(&ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
			Names: []*ast.Ident{id("promptReturned")}, Type: &ast.FuncType{Params: fields()}}}})
		f.line()
	}
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
			body = append(body, unmarshalValidate(f, dot("a", "conn"), mi.Notif, gate())...)
			// A prompt turn's cancel first cancels the session's in-flight prompt context.
			if mi.CancelsPromptTurn {
				body = append(body, cancelActivePrompt(mi.SessionField)...)
			}
			pre, recv := agentAssert(f, m.InterfaceOf(mi.Binding), mi.GoName, mi.Notif, "", false)
			body = append(body, pre...)
			body = append(body, callNoResult(recv, mi.GoName)...)
		} else {
			body = append(body, unmarshalValidate(f, dot("a", "conn"), mi.Req, gate())...)
			pre, recv := agentAssert(f, m.InterfaceOf(mi.Binding), mi.GoName, mi.Req, mi.Resp, mi.HasResult)
			body = append(body, pre...)
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
	emitOutbound(f, "AgentSideConnection", m.ClientMethods, m.Initialize != nil)

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
			body = append(body, unmarshalValidate(f, dot("c", "conn"), mi.Notif, gate())...)
			pre, recv := clientAssert(f, m.InterfaceOf(mi.Binding), mi.GoName, mi.Notif, "", false)
			body = append(body, pre...)
			body = append(body, callNoResult(recv, mi.GoName)...)
		} else {
			body = append(body, unmarshalValidate(f, dot("c", "conn"), mi.Req, gate())...)
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
	emitOutbound(f, "ClientSideConnection", m.AgentMethods, m.Initialize != nil)

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
		exprStmt(call(dot("a", "negotiation", "Store"), addr(composite(id("Negotiation"), kv(id("Request"), id("p")), kv(id("Response"), id("resp")))))),
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
		exprStmt(call(dot("c", "negotiation", "Store"), addr(composite(id("Negotiation"), kv(id("Request"), id("params")), kv(id("Response"), id("resp")))))),
		ret(id("resp"), nilExpr()),
	}
}

// emitOutbound emits one method per peer method on recvType that sends the
// request or notification over c.conn.
func emitOutbound(f *goFile, recvType string, methods []*model.Method, gated bool) {
	recv := func() *ast.Field { return param("c", star(id(recvType))) }
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
		return []ast.Stmt{ifStmt(define(exprs(id("err")), call(dot("c", "gate"), id(mi.Const), addr(id("params")))), errNotNil(), fail)}
	}
	// resultArg is the gate the response is checked by.
	resultArg := func() ast.Expr {
		if gated {
			return dot("c", "resultGate")
		}
		return nilExpr()
	}

	for _, mi := range methods {
		// A blank line separates each wrapper from the declaration before it.
		f.line()
		if mi.Notif != "" && mi.CancelsPending {
			// The cancel is written before the pending requests are answered
			// (0017-MADR-remediate-acp-v1-conformance-violations.md, Q1).
			f.add(method(recv(), mi.GoName, funcType(params(mi.Notif), param("", id("error"))),
				append(gateFirst(mi, nil),
					define(exprs(id("err")), call(dot("c", "conn", "SendNotification"), id("ctx"), id(mi.Const), id("params"))),
					exprStmt(call(dot("c", "pending", "cancel"), call(id("string"), dot("params", mi.SessionField)))),
					ret(id("err")),
				)...,
			))
			continue
		}
		if mi.Notif != "" {
			f.add(method(recv(), mi.GoName, funcType(params(mi.Notif), param("", id("error"))),
				append(gateFirst(mi, nil), ret(call(dot("c", "conn", "SendNotification"), id("ctx"), id(mi.Const), id("params"))))...,
			))
			continue
		}
		if !mi.HasResult {
			f.add(method(recv(), mi.GoName, funcType(params(mi.Req), param("", id("error"))),
				append(gateFirst(mi, nil), ret(call(dot("c", "conn", "SendRequestNoResult"), id("ctx"), id(mi.Const), id("params"))))...,
			))
			continue
		}
		if mi.Initializes {
			f.add(method(recv(), mi.GoName, funcType(params(mi.Req), param("", id(mi.Resp)), param("", id("error"))),
				initializeWrapper(mi)...))
			continue
		}
		body := append(gateFirst(mi, composite(id(mi.Resp))),
			define(exprs(id("resp"), id("err")),
				call(index(id("sendRequest"), id(mi.Resp)), dot("c", "conn"), id("ctx"), id(mi.Const), id("params"), resultArg())),
		)
		// A prompt turn whose context ended sends its cancel, best effort.
		if c := mi.Canceller; mi.PromptTurn && c != nil {
			body = append(body, ifStmt(nil, errNotNil(),
				ifStmt(nil, binary(call(dot("ctx", "Err")), token.NEQ, nilExpr()),
					assign(exprs(id("_")), call(dot("c", c.GoName),
						call(f.qual(pkgContext, "Background")),
						composite(id(c.Notif), kv(id(c.SessionField), dot("params", mi.SessionField))),
					)),
				),
			))
		}
		body = append(body, ret(id("resp"), id("err")))
		f.add(method(recv(), mi.GoName, funcType(params(mi.Req), param("", id(mi.Resp)), param("", id("error"))), body...))
	}
}
