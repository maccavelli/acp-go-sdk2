package emit

import (
	"go/ast"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/ir"
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
	f.add(constDecl(false, constSpec("ProtocolVersionNumber", nil, intLit(m.Meta.Version))))

	f.comment("Agent method names")
	f.add(methodConsts(m.Meta.AgentMethods, m.AgentMethodConsts))

	f.comment("Client method names")
	f.add(methodConsts(m.Meta.ClientMethods, m.ClientMethodConsts))

	return f.write(outDir, "constants_gen.go")
}

// methodConsts declares <const> = "wire" for each method, sorted by key, with
// the constant names the model gave them.
func methodConsts(methods, consts map[string]string) *ast.GenDecl {
	var specs []*ast.ValueSpec
	for _, k := range ir.SortedKeys(methods) {
		specs = append(specs, constSpec(consts[k], nil, str(methods[k])))
	}
	return constDecl(true, specs...)
}
