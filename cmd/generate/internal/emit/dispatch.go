package emit

import (
	"go/ast"
	"go/token"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// WriteDispatch emits agent_gen.go and client_gen.go with handlers and wrappers.
func WriteDispatch(outDir string, m *model.Model) error {
	if err := writeAgentGen(outDir, m); err != nil {
		return err
	}
	return writeClientGen(outDir, m)
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

func writeAgentGen(outDir string, m *model.Model) error {
	f := newFile()

	var cases []ast.Stmt
	for _, mi := range m.AgentMethods {
		var body []ast.Stmt
		if mi.Notif != "" {
			body = append(body, unmarshalValidate(f, mi.Notif)...)
			// session/cancel also cancels any in-flight prompt ctx for the session.
			if mi.Wire == wireSessionCancel {
				body = append(body, cancelActivePrompt()...)
			}
			pre, recv := agentAssert(f, mi.Binding, mi.GoName, mi.Notif, "", false)
			body = append(body, pre...)
			body = append(body, callNoResult(recv, mi.GoName)...)
		} else {
			body = append(body, unmarshalValidate(f, mi.Req)...)
			pre, recv := agentAssert(f, mi.Binding, mi.GoName, mi.Req, mi.Resp, mi.HasResult)
			body = append(body, pre...)
			switch {
			case mi.Wire == "session/prompt":
				body = append(body, promptCase(f, recv, mi.GoName)...)
			case !mi.HasResult:
				body = append(body, callNoResult(recv, mi.GoName)...)
			default:
				body = append(body, callWithResult(recv, mi.GoName)...)
			}
		}
		cases = append(cases, caseOf(exprs(id(mi.Const)), body...))
	}
	f.add(handleFunc(f, "a", "AgentSideConnection", cases))

	// Agent outbound wrappers (agent -> client).
	emitOutbound(f, "AgentSideConnection", m.ClientMethods)

	return f.write(outDir, "agent_gen.go")
}

func writeClientGen(outDir string, m *model.Model) error {
	f := newFile()

	var cases []ast.Stmt
	for _, mi := range m.ClientMethods {
		var body []ast.Stmt
		if mi.Notif != "" {
			body = append(body, unmarshalValidate(f, mi.Notif)...)
			pre, recv := clientAssert(f, mi.Binding, mi.GoName, mi.Notif, "", false)
			body = append(body, pre...)
			body = append(body, callNoResult(recv, mi.GoName)...)
		} else {
			body = append(body, unmarshalValidate(f, mi.Req)...)
			pre, recv := clientAssert(f, mi.Binding, mi.GoName, mi.Req, mi.Resp, mi.HasResult)
			body = append(body, pre...)
			if mi.HasResult {
				body = append(body, callWithResult(recv, mi.GoName)...)
			} else {
				body = append(body, callNoResult(recv, mi.GoName)...)
			}
		}
		cases = append(cases, caseOf(exprs(id(mi.Const)), body...))
	}
	f.add(handleFunc(f, "c", "ClientSideConnection", cases))

	// Client outbound wrappers (client -> agent).
	emitOutbound(f, "ClientSideConnection", m.AgentMethods)

	return f.write(outDir, "client_gen.go")
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
		// session/prompt from the client: if ctx was canceled, send session/cancel best-effort.
		if mi.Side == "agent" && mi.Wire == "session/prompt" {
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
		f.add(method(recv(), mi.GoName, funcType(params(mi.Req), param("", id(mi.Resp)), param("", id("error"))), body...))
	}
}
