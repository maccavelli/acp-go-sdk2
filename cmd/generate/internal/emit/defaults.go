package emit

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// valueExpr writes v, a schema default the model rendered as a Go value
// (0019-PLAN-idioms-performance.md, D3). An item of a slice or a map elides
// its composite type, as gofmt -s writes it. A ValueZero writes nothing, so
// it has no expression.
func valueExpr(f *goFile, v model.Value, item bool) ast.Expr {
	switch v.Kind {
	case model.ValueNil:
		return nilExpr()
	case model.ValueBool, model.ValueConst:
		return id(v.Text)
	case model.ValueString:
		return &ast.BasicLit{Kind: token.STRING, Value: v.Text}
	case model.ValueNumber:
		return numberLit(v.Text)
	case model.ValueStruct:
		return composite(nil)
	case model.ValueSlice, model.ValueMap:
		var typ ast.Expr
		if !item {
			typ = typeAST(f, v.Type)
		}
		var elts []ast.Expr
		for i, it := range v.Items {
			x := valueExpr(f, it, true)
			if v.Kind == model.ValueMap {
				x = kv(str(v.Keys[i]), x)
			}
			elts = append(elts, x)
		}
		return composite(typ, elts...)
	case model.ValuePointer:
		return call(id("new"), typedExpr(f, v.Type, v.Items[0]))
	case model.ValueRaw:
		return call(typeAST(f, v.Type), str(v.Text))
	}
	f.fail(fmt.Errorf("a default of value kind %d has no Go expression", v.Kind))
	return nilExpr()
}

// typedExpr writes v, a value of t, as an expression of type t, such as the
// operand of new: a composite of t's type, a typed constant as it is, and a
// literal converted to t unless it is already of t's default type.
func typedExpr(f *goFile, t model.TypeRef, v model.Value) ast.Expr {
	switch v.Kind {
	case model.ValueStruct:
		return composite(typeAST(f, t))
	case model.ValueSlice, model.ValueMap, model.ValuePointer, model.ValueRaw, model.ValueConst:
		return valueExpr(f, v, false)
	}
	x := valueExpr(f, v, true)
	if t.Kind == model.RefNamed && t.Name == untypedDefault(v) {
		return x
	}
	return call(typeAST(f, t), x)
}

// untypedDefault is the Go type an untyped constant of v takes by default.
func untypedDefault(v model.Value) string {
	switch v.Kind {
	case model.ValueBool:
		return "bool"
	case model.ValueString:
		return "string"
	case model.ValueNumber:
		if numberLit(v.Text).Kind == token.FLOAT {
			return "float64"
		}
		return "int"
	}
	return ""
}

// numberLit is the number literal text, an integer unless it has a fraction
// or an exponent.
func numberLit(text string) *ast.BasicLit {
	if strings.ContainsAny(text, ".eE") {
		return &ast.BasicLit{Kind: token.FLOAT, Value: text}
	}
	return &ast.BasicLit{Kind: token.INT, Value: text}
}
