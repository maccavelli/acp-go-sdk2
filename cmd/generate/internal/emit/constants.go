package emit

import (
	"go/ast"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

const (
	schemaTypeObject  = "object"
	schemaTypeArray   = "array"
	schemaTypeString  = "string"
	schemaTypeInteger = "integer"
	schemaTypeNumber  = "number"
	schemaTypeBoolean = "boolean"
	schemaTypeNull    = "null"
	jsonStructTag     = "json"
	unionContentBlock = "ContentBlock"
	wireSessionCancel = "session/cancel"
	nameSessionUpdate = "SessionUpdate"
	generatedFileMode = 0o600

	pkgContext = "context"
	pkgErrors  = "errors"
	pkgFmt     = "fmt"
	pkgJSON    = "encoding/json"
)

// WriteConstants writes the version and method constants to constants_gen.go.
func WriteConstants(outDir string, m *model.Model) error {
	f := newFile()
	f.comment("ProtocolVersionNumber is the ACP protocol version supported by this SDK.")
	f.add(constDecl(false, constSpec("ProtocolVersionNumber", nil, intLit(m.Version))))

	f.comment("Agent method names")
	f.add(methodConsts(m.AgentMethods))

	f.comment("Client method names")
	f.add(methodConsts(m.ClientMethods))

	return f.write(outDir, "constants_gen.go")
}

// methodConsts declares <const> = "wire" for each method, in the model's
// order, sorted by meta key.
func methodConsts(methods []*model.Method) *ast.GenDecl {
	var specs []*ast.ValueSpec
	for _, mi := range methods {
		specs = append(specs, constSpec(mi.Const, nil, str(mi.Wire)))
	}
	return constDecl(true, specs...)
}
