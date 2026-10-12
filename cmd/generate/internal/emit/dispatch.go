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
func promptCase(f *goFile, recv ast.Expr, mi *model.Method) []ast.Stmt {
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
		checkedResult(dot("a", "conn")),
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
	// promptTurns is true when the side holds prompt-turn contexts.
	promptTurns bool
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
		peer: "client",
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
		peer: "agent",
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
		"copy.",
	)
	f.add(method(recv(), "Connection", funcType(nil, param("", star(id("Connection")))), ret(dot("c", "conn"))))
	f.line()
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

// agentFile builds agent_gen.go: the agent's handler and its outbound wrappers.
func agentFile(m *model.Model) *goFile {
	f := newFile()
	emitSideConnection(f, agentSide(m))

	var cases []ast.Stmt
	for _, mi := range m.AgentMethods {
		var body []ast.Stmt
		if mi.Notif != "" {
			body = append(body, unmarshalValidate(f, dot("a", "conn"), mi.Notif)...)
			// A prompt turn's cancel first cancels the session's in-flight prompt context.
			if mi.CancelsPromptTurn {
				body = append(body, cancelActivePrompt(mi.SessionField)...)
			}
			pre, recv := agentAssert(f, m.InterfaceOf(mi.Binding), mi.GoName, mi.Notif, "", false)
			body = append(body, pre...)
			body = append(body, callNoResult(recv, mi.GoName)...)
		} else {
			body = append(body, unmarshalValidate(f, dot("a", "conn"), mi.Req)...)
			pre, recv := agentAssert(f, m.InterfaceOf(mi.Binding), mi.GoName, mi.Req, mi.Resp, mi.HasResult)
			body = append(body, pre...)
			switch {
			case mi.PromptTurn:
				body = append(body, promptCase(f, recv, mi)...)
			case !mi.HasResult:
				body = append(body, callNoResult(recv, mi.GoName)...)
			default:
				body = append(body, callWithResult(dot("a", "conn"), recv, mi.GoName)...)
			}
		}
		cases = append(cases, caseOf(exprs(id(mi.Const)), body...))
	}
	f.add(handleFunc(f, "a", "AgentSideConnection", cases))

	// Agent outbound wrappers (agent -> client).
	emitOutbound(f, "AgentSideConnection", m.ClientMethods)

	return f
}

// clientFile builds client_gen.go: the client's handler and its outbound wrappers.
func clientFile(m *model.Model) *goFile {
	f := newFile()
	emitSideConnection(f, clientSide(m))

	var cases []ast.Stmt
	for _, mi := range m.ClientMethods {
		var body []ast.Stmt
		if mi.Notif != "" {
			body = append(body, unmarshalValidate(f, dot("c", "conn"), mi.Notif)...)
			pre, recv := clientAssert(f, m.InterfaceOf(mi.Binding), mi.GoName, mi.Notif, "", false)
			body = append(body, pre...)
			body = append(body, callNoResult(recv, mi.GoName)...)
		} else {
			body = append(body, unmarshalValidate(f, dot("c", "conn"), mi.Req)...)
			pre, recv := clientAssert(f, m.InterfaceOf(mi.Binding), mi.GoName, mi.Req, mi.Resp, mi.HasResult)
			body = append(body, pre...)
			if mi.HasResult {
				body = append(body, callWithResult(dot("c", "conn"), recv, mi.GoName)...)
			} else {
				body = append(body, callNoResult(recv, mi.GoName)...)
			}
		}
		cases = append(cases, caseOf(exprs(id(mi.Const)), body...))
	}
	f.add(handleFunc(f, "c", "ClientSideConnection", cases))

	// Client outbound wrappers (client -> agent).
	emitOutbound(f, "ClientSideConnection", m.AgentMethods)

	return f
}

// emitOutbound emits one method per peer method on recvType that sends the
// request or notification over c.conn.
func emitOutbound(f *goFile, recvType string, methods []*model.Method) {
	recv := func() *ast.Field { return param("c", star(id(recvType))) }
	params := func(typeName string) []*ast.Field {
		return []*ast.Field{param("ctx", f.qual(pkgContext, "Context")), param("params", id(typeName))}
	}

	for _, mi := range methods {
		// A blank line separates each wrapper from the declaration before it.
		f.line()
		if mi.Notif != "" {
			f.add(method(recv(), mi.GoName, funcType(params(mi.Notif), param("", id("error"))),
				ret(call(dot("c", "conn", "SendNotification"), id("ctx"), id(mi.Const), id("params"))),
			))
			continue
		}
		if !mi.HasResult {
			f.add(method(recv(), mi.GoName, funcType(params(mi.Req), param("", id("error"))),
				ret(call(dot("c", "conn", "SendRequestNoResult"), id("ctx"), id(mi.Const), id("params"))),
			))
			continue
		}
		body := []ast.Stmt{
			define(exprs(id("resp"), id("err")),
				call(index(id("SendRequest"), id(mi.Resp)), dot("c", "conn"), id("ctx"), id(mi.Const), id("params"))),
		}
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
