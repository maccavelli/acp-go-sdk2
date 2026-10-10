package emit

import (
	"go/ast"
	"strings"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/ir"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/util"
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
func WriteConstants(outDir string, meta *load.Meta) error {
	f := newFile()
	f.comment("ProtocolVersionNumber is the ACP protocol version supported by this SDK.")
	f.add(constDecl(false, constSpec("ProtocolVersionNumber", nil, intLit(meta.Version))))

	f.comment("Agent method names")
	f.add(methodConsts("AgentMethod", meta.AgentMethods))

	f.comment("Client method names")
	f.add(methodConsts("ClientMethod", meta.ClientMethods))

	return f.write(outDir, "constants_gen.go")
}

// methodConsts declares prefix+Name = "wire" for each method, sorted by key.
func methodConsts(prefix string, methods map[string]string) *ast.GenDecl {
	var specs []*ast.ValueSpec
	for _, k := range ir.SortedKeys(methods) {
		specs = append(specs, constSpec(prefix+toExportedConst(k), nil, str(methods[k])))
	}
	return constDecl(true, specs...)
}

// toExportedConst turns a meta method key such as session_set_mode into
// SessionSetMode.
func toExportedConst(s string) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		parts[i] = util.TitleWord(parts[i])
	}
	return strings.Join(parts, "")
}
