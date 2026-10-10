package emit

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/ir"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// WriteDispatch emits agent_gen.go and client_gen.go with handlers and wrappers.
func WriteDispatch(outDir string, m *model.Model) error {
	groups := ir.BuildMethodGroups(m.Schema, m.Meta)

	if err := writeAgentGen(outDir, groups, m); err != nil {
		return err
	}
	return writeClientGen(outDir, groups, m)
}

// sessionKey is string(p.SessionId).
func sessionKey() ast.Expr { return call(id("string"), dot("p", "SessionId")) }

// sessionCancel is a.sessionCancels[string(p.SessionId)].
func sessionCancel() ast.Expr { return index(dot("a", "sessionCancels"), sessionKey()) }

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

// promptCase derives a cancellable context per session prompt, calls Prompt,
// then drops the cancel entry.
func promptCase(f *goFile, recv ast.Expr, methodName string) []ast.Stmt {
	return []ast.Stmt{
		varStmt("reqCtx", f.qual(pkgContext, "Context")),
		varStmt("cancel", f.qual(pkgContext, "CancelFunc")),
		assign(exprs(id("reqCtx"), id("cancel")), call(f.qual(pkgContext, "WithCancel"), id("ctx"))),
		lockCall("Lock"),
		ifStmt(define(exprs(id("prev"), id("ok")), sessionCancel()), id("ok"), exprStmt(call(id("prev")))),
		assign(exprs(sessionCancel()), id("cancel")),
		lockCall("Unlock"),
		define(exprs(id("resp"), id("err")), call(sel(recv, methodName), id("reqCtx"), id("p"))),
		lockCall("Lock"),
		exprStmt(call(id("delete"), dot("a", "sessionCancels"), sessionKey())),
		lockCall("Unlock"),
		ifStmt(nil, errNotNil(), toRequestError()),
		ret(id("resp"), nilExpr()),
	}
}

// cancelActivePrompt cancels the in-flight prompt context of the session, if any.
func cancelActivePrompt() []ast.Stmt {
	return []ast.Stmt{
		lockCall("Lock"),
		ifStmt(define(exprs(id("cn"), id("ok")), sessionCancel()), id("ok"),
			exprStmt(call(id("cn"))),
			exprStmt(call(id("delete"), dot("a", "sessionCancels"), sessionKey())),
		),
		lockCall("Unlock"),
	}
}

func writeAgentGen(outDir string, groups ir.Groups, m *model.Model) error {
	schema, meta := m.Schema, m.Meta
	f := newFile()

	var cases []ast.Stmt
	for _, k := range ir.SortedKeys(meta.AgentMethods) {
		mi := groups["agent|"+meta.AgentMethods[k]]
		if mi == nil {
			continue
		}
		var body []ast.Stmt
		if mi.Notif != "" {
			body = append(body, unmarshalValidate(f, mi.Notif)...)
			// session/cancel also cancels any in-flight prompt ctx for the session.
			if mi.Method == wireSessionCancel {
				body = append(body, cancelActivePrompt()...)
			}
			callName := ir.DispatchMethodNameForNotification(k, mi.Notif)
			pre, recv := agentAssert(f, mi.Binding, callName, mi.Notif, "", false)
			body = append(body, pre...)
			body = append(body, callNoResult(recv, callName)...)
		} else if mi.Req != "" {
			respName := strings.TrimSuffix(mi.Req, "Request") + "Response"
			nullResp := ir.IsNullResponse(schema.Defs[respName])
			body = append(body, unmarshalValidate(f, mi.Req)...)
			methodName := strings.TrimSuffix(mi.Req, "Request")
			pre, recv := agentAssert(f, mi.Binding, methodName, mi.Req, respName, !nullResp)
			body = append(body, pre...)
			switch {
			case mi.Method == "session/prompt":
				body = append(body, promptCase(f, recv, methodName)...)
			case nullResp:
				body = append(body, callNoResult(recv, methodName)...)
			default:
				body = append(body, callWithResult(recv, methodName)...)
			}
		}
		if len(body) > 0 {
			cases = append(cases, caseOf(exprs(id(m.AgentMethodConsts[k])), body...))
		}
	}
	f.add(handleFunc(f, "a", "AgentSideConnection", cases))

	// Agent outbound wrappers (agent -> client).
	emitOutbound(f, "AgentSideConnection", "client", groups, m, meta.ClientMethods, m.ClientMethodConsts)

	return f.write(outDir, "agent_gen.go")
}

func writeClientGen(outDir string, groups ir.Groups, m *model.Model) error {
	schema, meta := m.Schema, m.Meta
	f := newFile()

	var cases []ast.Stmt
	for _, k := range ir.SortedKeys(meta.ClientMethods) {
		mi := groups["client|"+meta.ClientMethods[k]]
		if mi == nil {
			continue
		}
		var body []ast.Stmt
		if mi.Notif != "" {
			body = append(body, unmarshalValidate(f, mi.Notif)...)
			callName := ir.DispatchMethodNameForNotification(k, mi.Notif)
			pre, recv := clientAssert(f, mi.Binding, callName, mi.Notif, "", false)
			body = append(body, pre...)
			body = append(body, callNoResult(recv, callName)...)
		} else if mi.Req != "" {
			respName := strings.TrimSuffix(mi.Req, "Request") + "Response"
			nullResp := ir.IsNullResponse(schema.Defs[respName])
			body = append(body, unmarshalValidate(f, mi.Req)...)
			methodName := strings.TrimSuffix(mi.Req, "Request")
			pre, recv := clientAssert(f, mi.Binding, methodName, mi.Req, respName, !nullResp)
			body = append(body, pre...)
			if nullResp {
				body = append(body, callNoResult(recv, methodName)...)
			} else {
				body = append(body, callWithResult(recv, methodName)...)
			}
		}
		if len(body) > 0 {
			cases = append(cases, caseOf(exprs(id(m.ClientMethodConsts[k])), body...))
		}
	}
	f.add(handleFunc(f, "c", "ClientSideConnection", cases))

	// Client outbound wrappers (client -> agent).
	emitOutbound(f, "ClientSideConnection", "agent", groups, m, meta.AgentMethods, m.AgentMethodConsts)

	return f.write(outDir, "client_gen.go")
}

// emitOutbound emits one method per peer method on recvType that sends the
// request or notification over c.conn. consts holds the constant name of
// each method key.
func emitOutbound(f *goFile, recvType, side string, groups ir.Groups, m *model.Model, methods, consts map[string]string) {
	constByWire := map[string]string{}
	for k, wire := range methods {
		constByWire[wire] = consts[k]
	}
	recv := func() *ast.Field { return param("c", star(id(recvType))) }
	params := func(typeName string) []*ast.Field {
		return []*ast.Field{param("ctx", f.qual(pkgContext, "Context")), param("params", id(typeName))}
	}

	for _, k := range ir.SortedKeys(methods) {
		mi := groups[side+"|"+methods[k]]
		if mi == nil {
			continue
		}
		constName := constByWire[mi.Method]
		if constName == "" {
			continue
		}
		// A blank line separates each wrapper from the declaration before it.
		f.line()
		if mi.Notif != "" {
			name := strings.TrimSuffix(mi.Notif, "Notification")
			switch mi.Method {
			case "session/update":
				name = nameSessionUpdate
			case wireSessionCancel:
				name = "Cancel"
			}
			f.add(method(recv(), name, funcType(params(mi.Notif), param("", id("error"))),
				ret(call(dot("c", "conn", "SendNotification"), id("ctx"), id(constName), id("params"))),
			))
			continue
		}
		if mi.Req == "" {
			continue
		}
		name := strings.TrimSuffix(mi.Req, "Request")
		respName := name + "Response"
		if ir.IsNullResponse(m.Schema.Defs[respName]) {
			f.add(method(recv(), name, funcType(params(mi.Req), param("", id("error"))),
				ret(call(dot("c", "conn", "SendRequestNoResult"), id("ctx"), id(constName), id("params"))),
			))
			continue
		}
		body := []ast.Stmt{
			define(exprs(id("resp"), id("err")),
				call(index(id("SendRequest"), id(respName)), dot("c", "conn"), id("ctx"), id(constName), id("params"))),
		}
		// session/prompt from the client: if ctx was canceled, send session/cancel best-effort.
		if side == "agent" && mi.Method == "session/prompt" {
			body = append(body, ifStmt(nil, errNotNil(),
				ifStmt(nil, binary(call(dot("ctx", "Err")), token.NEQ, nilExpr()),
					assign(exprs(id("_")), call(dot("c", "Cancel"),
						call(f.qual(pkgContext, "Background")),
						composite(id("CancelNotification"), kv(id("SessionId"), dot("params", "SessionId"))),
					)),
				),
			))
		}
		body = append(body, ret(id("resp"), id("err")))
		f.add(method(recv(), name, funcType(params(mi.Req), param("", id(respName)), param("", id("error"))), body...))
	}
}
