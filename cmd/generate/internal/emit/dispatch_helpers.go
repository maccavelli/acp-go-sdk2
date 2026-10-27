package emit

import (
	"go/ast"
	"go/token"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// invalidParams returns nil, NewInvalidParams(map[string]any{"error": err.Error()}).
func invalidParams() ast.Stmt {
	return ret(nilExpr(), call(id("NewInvalidParams"),
		composite(rawMap(id("any")), kv(str("error"), call(dot("err", "Error")))),
	))
}

// toRequestError returns nil, toReqErr(err).
func toRequestError() ast.Stmt { return ret(nilExpr(), call(id("toReqErr"), id("err"))) }

// methodNotFound returns nil, NewMethodNotFound(method).
func methodNotFound() ast.Stmt { return ret(nilExpr(), call(id("NewMethodNotFound"), id("method"))) }

// errNotNil is err != nil.
func errNotNil() ast.Expr { return binary(id("err"), token.NEQ, nilExpr()) }

// kindCheck refuses a method received as the other kind: a request sent
// without an id is not handled, and nothing answers it; a notification sent
// with an id is answered method not found, as no request of that name exists
// (0019-PLAN-behaviour-fixes.md, B12).
func kindCheck(notification bool) ast.Stmt {
	if notification {
		return ifStmt(nil, call(id("sentAsRequest"), id("ctx")), methodNotFound())
	}
	return ifStmt(nil, call(id("sentAsNotification"), id("ctx")),
		ret(nilExpr(), call(id("NewInvalidRequest"), composite(rawMap(id("any")),
			kv(str("error"), binary(id("method"), token.ADD, str(" is a request, and it was sent without an id")))))))
}

// unmarshalValidate decodes params into p and validates it. Params left out
// decode as an empty object (0019-PLAN-behaviour-fixes.md, B3).
func unmarshalValidate(conn ast.Expr, typeName string, gate ast.Expr) []ast.Stmt {
	return []ast.Stmt{
		varStmt("p", id(typeName)),
		ifStmt(define(exprs(id("err")), call(id("decodeParams"), id("params"), addr(id("p")))), errNotNil(),
			invalidParams()),
		// The receiving order: the consumer's rewrite hook, validation, the
		// validation hook (0015-MADR-validate-absolute-protocol-paths.md, H1).
		ifStmt(define(exprs(id("err")), call(sel(conn, "checkInbound"), id("ctx"), id("method"), addr(id("p")), gate)), errNotNil(),
			ret(nilExpr(), call(id("inboundError"), id("err")))),
	}
}

// agentAssert returns the interface assertion prelude for the interface a
// method is called through and the receiver the method is called on.
func agentAssert(f *goFile, in model.Interface, methodName, paramType, respType string, hasResponse bool) ([]ast.Stmt, ast.Expr) {
	switch in.Binding {
	case model.BindAgentOptional:
		return namedAssert(dot("a", "agent"), in.Var, in.Name), id(in.Var)
	case model.BindAgentExperimental:
		return singleMethodAssert(f, dot("a", "agent"), in.Var, methodName, paramType, respType, hasResponse), id(in.Var)
	default:
		return nil, dot("a", "agent")
	}
}

// clientAssert returns the interface assertion prelude for the interface a
// method is called through and the receiver the method is called on.
func clientAssert(f *goFile, in model.Interface, methodName, paramType, respType string, hasResponse bool) ([]ast.Stmt, ast.Expr) {
	switch in.Binding {
	case model.BindClientExperimental:
		return singleMethodAssert(f, dot("c", "client"), in.Var, methodName, paramType, respType, hasResponse), id(in.Var)
	default:
		return nil, dot("c", "client")
	}
}

// namedAssert is name, ok := receiver.(iface); if !ok { return method not found }.
func namedAssert(receiver ast.Expr, name, iface string) []ast.Stmt {
	return []ast.Stmt{
		define(exprs(id(name), id("ok")), typeAssert(receiver, id(iface))),
		ifStmt(nil, not(id("ok")), methodNotFound()),
	}
}

// singleMethodAssert asserts receiver to an anonymous one-method interface.
func singleMethodAssert(f *goFile, receiver ast.Expr, name, methodName, paramType, respType string, hasResponse bool) []ast.Stmt {
	res := []*ast.Field{param("", id("error"))}
	if hasResponse {
		res = []*ast.Field{param("", id(respType)), param("", id("error"))}
	}
	iface := &ast.InterfaceType{Methods: fields(&ast.Field{
		Names: []*ast.Ident{id(methodName)},
		Type:  funcType([]*ast.Field{param("", f.qual(pkgContext, "Context")), param("", id(paramType))}, res...),
	})}
	return []ast.Stmt{
		define(exprs(id(name), id("ok")), typeAssert(receiver, iface)),
		ifStmt(nil, not(id("ok")), methodNotFound()),
	}
}

// recvCall is recv.method(ctx, p).
func recvCall(recv ast.Expr, methodName string) *ast.CallExpr {
	return call(sel(recv, methodName), id("ctx"), id("p"))
}

// callNoResult calls a method that returns only an error.
func callNoResult(recv ast.Expr, methodName string) []ast.Stmt {
	return []ast.Stmt{
		ifStmt(define(exprs(id("err")), recvCall(recv, methodName)), errNotNil(), toRequestError()),
		ret(nilExpr(), nilExpr()),
	}
}

// callWithResult calls a method that returns a response and an error, and
// returns the response through checkedResult.
func callWithResult(conn, recv ast.Expr, methodName string, gate ast.Expr) []ast.Stmt {
	return []ast.Stmt{
		define(exprs(id("resp"), id("err")), recvCall(recv, methodName)),
		ifStmt(nil, errNotNil(), toRequestError()),
		checkedResult(conn, gate),
	}
}

// checkedResult is return conn.checkResult(method, &p, &resp): a result that
// fails validation as sent is answered -32603 instead
// (0015-MADR-validate-absolute-protocol-paths.md, E1), and so is one the
// connection's result check for the method refuses against the params
// (0040-MADR-check-elicitation-forms-answers-and-urls.md, item 6). Pointers,
// so nothing is copied (0019-PLAN-idioms-performance.md, D6).
func checkedResult(conn, gate ast.Expr) ast.Stmt {
	return ret(call(sel(conn, "checkResult"), id("method"), addr(id("p")), addr(id("resp")), gate))
}
