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

// unmarshalValidate decodes params into p and validates it.
func unmarshalValidate(f *goFile, typeName string) []ast.Stmt {
	return []ast.Stmt{
		varStmt("p", id(typeName)),
		ifStmt(define(exprs(id("err")), call(f.qual(pkgJSON, "Unmarshal"), id("params"), addr(id("p")))), errNotNil(),
			invalidParams()),
		ifStmt(define(exprs(id("err")), call(dot("p", "Validate"))), errNotNil(),
			invalidParams()),
	}
}

// agentAssert returns the interface assertion prelude for a binding and the
// receiver the method is called on.
func agentAssert(f *goFile, binding model.Binding, methodName, paramType, respType string, hasResponse bool) ([]ast.Stmt, ast.Expr) {
	switch binding {
	case model.BindAgentLoader:
		return namedAssert(dot("a", "agent"), "loader", "AgentLoader"), id("loader")
	case model.BindAgentExperimental:
		return singleMethodAssert(f, dot("a", "agent"), "exp", methodName, paramType, respType, hasResponse), id("exp")
	default:
		return nil, dot("a", "agent")
	}
}

// clientAssert returns the interface assertion prelude for a binding and the
// receiver the method is called on.
func clientAssert(f *goFile, binding model.Binding, methodName, paramType, respType string, hasResponse bool) ([]ast.Stmt, ast.Expr) {
	switch binding {
	case model.BindClientExperimental:
		return singleMethodAssert(f, dot("c", "client"), "exp", methodName, paramType, respType, hasResponse), id("exp")
	case model.BindClientTerminal:
		return namedAssert(dot("c", "client"), "t", "ClientTerminal"), id("t")
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

// callWithResult calls a method that returns a response and an error.
func callWithResult(recv ast.Expr, methodName string) []ast.Stmt {
	return []ast.Stmt{
		define(exprs(id("resp"), id("err")), recvCall(recv, methodName)),
		ifStmt(nil, errNotNil(), toRequestError()),
		ret(id("resp"), nilExpr()),
	}
}
