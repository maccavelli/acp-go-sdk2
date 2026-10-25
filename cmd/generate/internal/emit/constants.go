package emit

import (
	"go/ast"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

const (
	schemaTypeNull = "null"
	jsonStructTag  = "json"
	// unknownField holds a discriminated union's unrecognized payload.
	unknownField = "Unknown"
	// extraField holds the keys a union's catch-all variant does not declare.
	extraField = "Extra"

	pkgBytes   = "bytes"
	pkgContext = "context"
	pkgErrors  = "errors"
	pkgFmt     = "fmt"
	pkgIO      = "io"
	pkgJSON    = "encoding/json"
	pkgSlog    = "log/slog"
	pkgStrings = "strings"
	pkgSync    = "sync"
	pkgAtomic  = "sync/atomic"
)

// constantsFile builds constants_gen.go: the version and method constants.
func constantsFile(m *model.Model) *goFile {
	f := newFile()
	f.comment("ProtocolVersionNumber is the ACP protocol version supported by this SDK.")
	f.add(constDecl(false, constSpec("ProtocolVersionNumber", nil, intLit(m.Version))))

	f.comment("Agent method names")
	f.add(methodConsts(m.AgentMethods))

	f.comment("Client method names")
	f.add(methodConsts(m.ClientMethods))

	if len(m.Protocol) > 0 {
		f.comment("Protocol method names")
		f.add(methodConsts(m.Protocol))
	}

	return f
}

// methodConsts declares <const> = "wire" for each method, in the model's
// order, sorted by meta key. The constant of a method the unstable manifest
// keeps carries its notice
// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 1).
func methodConsts(methods []*model.Method) *ast.GenDecl {
	var specs []*ast.ValueSpec
	for _, mi := range methods {
		s := constSpec(mi.Const, nil, str(mi.Wire))
		if d := mi.Deprecation; d != nil && d.Frozen {
			s.Doc = docLines(methodNotice(nil, mi)...)
		}
		specs = append(specs, s)
	}
	return constDecl(true, specs...)
}
