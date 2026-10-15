package model

import (
	"fmt"
	"go/token"
	"go/types"
	"maps"
	"slices"
	"strings"
	"unicode"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/util"
)

// namer hands out Go names. typeNames holds every type name; the nested-type
// heuristics avoid them, as they did when the types emitter named types. pkg
// holds every package-level name the generator declares, with the place that
// declared it, so a second declaration is an error.
type namer struct {
	typeNames map[string]bool
	pkg       map[string]string
}

func newNamer() *namer {
	return &namer{typeNames: map[string]bool{}, pkg: map[string]string{}}
}

// declare records a package-level name for owner.
func (n *namer) declare(name, owner string) error {
	if err := checkExported(name); err != nil {
		return fmt.Errorf("%s: %w", owner, err)
	}
	if prev, ok := n.pkg[name]; ok {
		return fmt.Errorf("%s: %s is already declared by %s", owner, name, prev)
	}
	n.pkg[name] = owner
	return nil
}

// declareType declares a type name and adds it to the names the nested-type
// heuristics avoid.
func (n *namer) declareType(name, owner string) error {
	if err := n.declare(name, owner); err != nil {
		return err
	}
	n.typeNames[name] = true
	return nil
}

// checkExported fails unless name is an exported Go identifier.
func checkExported(name string) error {
	if !token.IsIdentifier(name) || !token.IsExported(name) {
		return fmt.Errorf("%q is not an exported Go identifier", name)
	}
	return nil
}

// fieldName is the Go field name of a schema property. It fails when the
// field would not be an exported identifier, when a json struct tag cannot
// carry the key (G9), and when the field would take the name of a method the
// generator emits (G14) (0019-PLAN-generator-hardening.md).
func fieldName(owner, key string) (string, error) {
	name := util.ToExportedField(key)
	if err := checkExported(name); err != nil {
		return "", fmt.Errorf("%s.properties.%s: field name: %w", owner, key, err)
	}
	if !tagKey(key) {
		return "", fmt.Errorf("%s.properties.%s: a json struct tag cannot carry this key", owner, key)
	}
	if err := checkNotMethod(name); err != nil {
		return "", fmt.Errorf("%s.properties.%s: %w", owner, key, err)
	}
	return name, nil
}

// generatedMethods are the methods the generator emits on a struct or a
// union; a field of one of these names would collide with it.
var generatedMethods = []string{"MarshalJSON", "UnmarshalJSON", "Validate"}

// checkNotMethod fails when a field name is that of a generated method.
func checkNotMethod(name string) error {
	if slices.Contains(generatedMethods, name) {
		return fmt.Errorf("field %s collides with the generated method %s", name, name)
	}
	return nil
}

// tagKey reports whether a json struct tag can carry key as its name, as
// encoding/json's Marshal documents: a non-empty string of Unicode letters,
// digits, and ASCII punctuation except quotation marks, backslash, and comma.
func tagKey(key string) bool {
	if key == "" {
		return false
	}
	for _, r := range key {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", r):
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			return false
		}
	}
	return true
}

// checkFields fails when two properties of one struct become the same field,
// or a property's field name is not an exported identifier.
func checkFields(owner string, props map[string]*load.Definition) error {
	seen := map[string]string{}
	for _, key := range slices.Sorted(maps.Keys(props)) {
		name, err := fieldName(owner, key)
		if err != nil {
			return err
		}
		if prev, ok := seen[name]; ok {
			return fmt.Errorf("%s: properties %q and %q both become field %s", owner, prev, key, name)
		}
		seen[name] = key
	}
	return nil
}

// paramName returns the constructor parameter name of a raw schema key
// (0014-MADR-union-constructors-take-every-required-key.md, N1, K2 and L1):
// the key itself when it is a Go identifier or keyword, otherwise the key's
// words in lower camel case (lowerCamel). A name that is a Go keyword or a
// predeclared identifier gets Value appended. It fails when no identifier
// results: a key with no letter or digit, or one that begins with a digit.
func paramName(owner, key string) (string, error) {
	name := key
	if !token.IsIdentifier(key) && !token.IsKeyword(key) {
		name = lowerCamel(key)
	}
	if token.IsKeyword(name) || types.Universe.Lookup(name) != nil {
		name += "Value"
	}
	if !token.IsIdentifier(name) {
		return "", fmt.Errorf("%s: required property %q cannot be a Go parameter name", owner, key)
	}
	return name, nil
}

// lowerCamel splits key at runs of characters that are not ASCII letters
// or digits, puts each word in Go case as util.ToExportedField does, and
// lowers the first word's leading capitals: URL-path is urlPath, x-API-key
// is xAPIKey, HTTPServer-url is httpServerUrl.
func lowerCamel(key string) string {
	words := strings.FieldsFunc(key, func(r rune) bool {
		return r > unicode.MaxASCII || (!unicode.IsLetter(r) && !unicode.IsDigit(r))
	})
	for i, w := range words {
		words[i] = util.ToExportedField(w)
	}
	if len(words) > 0 {
		words[0] = lowerLead(words[0])
	}
	return strings.Join(words, "")
}

// lowerLead lowers the leading run of capitals of s, except a capital that
// a lower-case letter follows, which begins the next word: URLPath is
// urlPath, ACMEId is acmeId, HTTP2Server is http2Server (Go Code Review
// Comments, "Initialisms"; oapi-codegen's LowercaseFirstCharacters).
func lowerLead(s string) string {
	r := []rune(s)
	for i := 0; i < len(r) && unicode.IsUpper(r[i]); i++ {
		if i > 0 && i+1 < len(r) && unicode.IsLower(r[i+1]) {
			break
		}
		r[i] = unicode.ToLower(r[i])
	}
	return string(r)
}

// generateNestedTypeName applies multiple heuristics to create idiomatic nested type names
// while guaranteeing stability (name depends only on parent, field, and collision check).
//
// If a collision is detected (e.g. the schema already defines a type with the same name we
// would otherwise generate), fall back to a deterministic suffixed name. It fails only if it
// cannot find a unique name.
func generateNestedTypeName(parentName, rawFieldName string, usedNames map[string]bool) (string, error) {
	// Heuristic 1: Ensure field name is properly capitalized first
	fieldName := util.ToExportedField(rawFieldName)

	// Heuristic 2: Try RPC suffix stripping for Request/Response/Notification parents
	// This handles cases like RequestPermissionRequest + ToolCall → RequestPermissionToolCall
	for _, suffix := range []string{"Request", "Response", "Notification"} {
		if baseName, ok := strings.CutSuffix(parentName, suffix); ok && baseName != "" {
			candidate := baseName + fieldName
			if !usedNames[candidate] {
				return candidate, nil
			}
		}
	}

	// Heuristic 3: Try word-boundary deduplication
	// This handles cases like AgentNotification + ExtNotification → AgentExtNotification
	parentWords := util.SplitCamel(parentName)
	fieldWords := util.SplitCamel(fieldName)

	// Need at least 2 words in parent to consider deduplication
	if len(parentWords) >= 2 && len(fieldWords) > 0 {
		lastParentWord := parentWords[len(parentWords)-1]
		firstFieldWord := fieldWords[0]
		lastFieldWord := fieldWords[len(fieldWords)-1]

		if strings.EqualFold(lastParentWord, firstFieldWord) || strings.EqualFold(lastParentWord, lastFieldWord) {
			// Remove last word from parent, then concatenate
			parentWithoutLast := strings.Join(parentWords[:len(parentWords)-1], "")
			if parentWithoutLast != "" {
				candidate := parentWithoutLast + fieldName
				if !usedNames[candidate] {
					return candidate, nil
				}
			}
		}
	}

	// Heuristic 4: Fall back to full concatenation
	fullName := parentName + fieldName
	if !usedNames[fullName] {
		return fullName, nil
	}

	// Collision: this can happen when the schema already defines a top-level type with the
	// same name as an inline property we'd otherwise generate.
	base := fullName + "Inline"
	if !usedNames[base] {
		return base, nil
	}
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s%d", base, i)
		if !usedNames[candidate] {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s: no unique nested type name for %q", parentName, rawFieldName)
}
