package emit

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// TestTypeDoc requires a type's doc to open with the line that names its
// place, before the description, unless the description already starts with
// the name (0024-PLAN-documentation.md, A7).
func TestTypeDoc(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		doc  []string
		want []string
	}{
		{"no description", nil, []string{"Foo is the Foo schema definition."}},
		{"a description", []string{"Says what.", "More."}, []string{"Foo is the Foo schema definition.", "", "Says what.", "More."}},
		{"named already", []string{"Foo holds things."}, []string{"Foo holds things."}},
		{"a longer name", []string{"Foobar holds things."}, []string{"Foo is the Foo schema definition.", "", "Foobar holds things."}},
	} {
		if got := typeDoc("Foo", "the Foo schema definition", tt.doc); !slices.Equal(got, tt.want) {
			t.Errorf("%s: typeDoc = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// TestOutboundErrors requires a typed outbound method's doc to name the errors
// its gates, its checks and the connection return, and only those, in lines
// of at most 76 characters (0024-PLAN-documentation.md, A6).
func TestOutboundErrors(t *testing.T) {
	t.Parallel()
	const (
		notInit   = "an error wrapping ErrNotInitialized until initialize completes"
		capErr    = "a *CapabilityError when what initialize agreed"
		termAuth  = "ErrTerminalAuthMethod"
		advAuth   = "ErrUnadvertisedAuthMethod"
		version   = "a *ProtocolVersionError when params ask for a version"
		valid     = "a *ValidationError when params break the schema"
		conn      = "a *RequestError wrapping context.Cause(ctx) when ctx ends, ErrPeerDisconnected or ErrConnectionClosed"
		answer    = "the peer's error answer as a *RequestError."
		gatedResp = "A response that breaks the schema or what initialize agreed returns an error wrapping *ValidationError or *CapabilityError"
		plainResp = "A response that breaks the schema returns an error wrapping *ValidationError,"
		undecoded = "one that does not decode a *RequestError."
		versionAn = "An answer with another protocol version returns a *ProtocolVersionError."
	)
	every := []string{notInit, capErr, termAuth, advAuth, version, valid, conn, answer, gatedResp, plainResp, undecoded, versionAn}
	for _, tt := range []struct {
		name  string
		mi    model.Method
		gated bool
		auth  authGates
		want  []string
	}{
		{"a gated notification", model.Method{Notif: "CancelNotification"}, true, authGates{},
			[]string{notInit, capErr, valid, conn}},
		{"a gated request with no result", model.Method{}, true, authGates{},
			[]string{notInit, capErr, valid, conn, answer}},
		{"a gated request", model.Method{HasResult: true}, true, authGates{},
			[]string{notInit, capErr, valid, conn, answer, gatedResp, undecoded}},
		{"a terminal-auth request", model.Method{HasResult: true}, true, authGates{terminal: true},
			[]string{notInit, capErr, termAuth, valid, conn, answer, gatedResp, undecoded}},
		{"an authenticate request", model.Method{HasResult: true}, true, authGates{terminal: true, advertised: true},
			[]string{notInit, capErr, termAuth, advAuth, valid, conn, answer, gatedResp, undecoded}},
		{"initialize", model.Method{HasResult: true, Initializes: true}, true, authGates{},
			[]string{version, valid, conn, answer, gatedResp, undecoded, versionAn}},
		{"an ungated request", model.Method{HasResult: true}, false, authGates{},
			[]string{valid, conn, answer, plainResp, undecoded}},
		{"an ungated notification", model.Method{Notif: "N"}, false, authGates{},
			[]string{valid, conn}},
	} {
		lines := outboundErrors(&tt.mi, tt.gated, tt.auth)
		for _, l := range lines {
			if len(l) > 76 {
				t.Errorf("%s: line of %d characters: %q", tt.name, len(l), l)
			}
		}
		text := strings.Join(lines, " ")
		for _, phrase := range every {
			if want := slices.Contains(tt.want, phrase); strings.Contains(text, phrase) != want {
				t.Errorf("%s: names %q: %v, want %v\n%s", tt.name, phrase, !want, want, text)
			}
		}
		if tt.mi.Notif != "" && !strings.HasSuffix(text, "or the write's error when writing fails.") {
			t.Errorf("%s: a notification's errors end %q", tt.name, text)
		}
	}
}

// TestListOf requires one item alone and the last of several after "; and".
func TestListOf(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		items []string
		want  string
	}{
		{[]string{"a"}, "a"},
		{[]string{"a", "b"}, "a; and b"},
		{[]string{"a", "b or c", "d"}, "a; b or c; and d"},
	} {
		if got := listOf(tt.items); got != tt.want {
			t.Errorf("listOf(%q) = %q, want %q", tt.items, got, tt.want)
		}
	}
}

// TestWrapDoc requires lines that hold the text's words in order, each of at
// most 76 characters unless it is one longer word, and each as full as the
// next line's first word allows.
func TestWrapDoc(t *testing.T) {
	t.Parallel()
	text := strings.Repeat("word ", 40) + "a-single-word-longer-than-seventy-six-characters-" + strings.Repeat("x", 40) + " end."
	lines := wrapDoc(text)
	for i, l := range lines {
		if len(l) > 76 && strings.Contains(l, " ") {
			t.Errorf("line of %d characters: %q", len(l), l)
		}
		if i+1 < len(lines) && len(l)+1+len(strings.Fields(lines[i+1])[0]) <= 76 {
			t.Errorf("line %d could hold the next word: %q", i, l)
		}
	}
	if got := strings.Join(lines, " "); got != strings.Join(strings.Fields(text), " ") {
		t.Errorf("wrapDoc lost words: %q", got)
	}
	if got := wrapDoc("short"); !slices.Equal(got, []string{"short"}) {
		t.Errorf("wrapDoc(short) = %q", got)
	}
}

// TestDiscDoc requires the sentence about a variant's discriminator field to
// name the field, the JSON key and the value the schema fixes, or, for a
// key-less variant of a union with a catch-all, to say the schema leaves it
// absent or null; and no sentence for any other variant
// (0056-PLAN-bring-the-docs-current-and-keep-them-current.md, phase 4b).
func TestDiscDoc(t *testing.T) {
	t.Parallel()
	inline := &model.Inline{Fields: []*model.Field{{Key: "kind", Name: "Kind"}, {Key: "x", Name: "X"}}}
	withCatchAll := &model.Type{DiscKey: "kind", DiscField: "Kind", CatchAll: 1}
	noCatchAll := &model.Type{DiscKey: "kind", DiscField: "Kind", CatchAll: -1}
	const (
		fixed  = `Its Kind field holds the "kind" discriminator, which the schema fixes at "a".`
		absent = `Its Kind field holds the "kind" discriminator, which the schema leaves absent or null for this variant.`
	)
	for _, tt := range []struct {
		name string
		t    *model.Type
		vi   *model.Variant
		want string
	}{
		{"a value", noCatchAll, &model.Variant{Inline: inline, DiscValue: "a",
			ConstPairs: [][2]string{{"kind", "a"}}, ConstFields: []string{"Kind"}}, fixed},
		{"a value after another constant", noCatchAll, &model.Variant{Inline: inline, DiscValue: "a",
			ConstPairs: [][2]string{{"aaa", "z"}, {"kind", "a"}}, ConstFields: []string{"Aaa", "Kind"}}, fixed},
		{"a value no string field holds", noCatchAll, &model.Variant{Inline: inline, DiscValue: "a",
			ConstPairs: [][2]string{{"kind", "a"}}}, ""},
		{"a variant with a $ref", noCatchAll, &model.Variant{DiscValue: "a",
			ConstPairs: [][2]string{{"kind", "a"}}, ConstFields: []string{"Kind"}}, ""},
		{"key-less, in a union with a catch-all", withCatchAll, &model.Variant{Inline: inline, KeyLess: true}, absent},
		{"key-less, in a union without one", noCatchAll, &model.Variant{Inline: inline, KeyLess: true}, ""},
		{"key-less, without the field", withCatchAll, &model.Variant{KeyLess: true,
			Inline: &model.Inline{Fields: []*model.Field{{Key: "x", Name: "X"}}}}, ""},
		{"the catch-all", withCatchAll, &model.Variant{Inline: inline}, ""},
		{"a union without a discriminator", &model.Type{CatchAll: -1}, &model.Variant{Inline: inline}, ""},
	} {
		if got := strings.Join(discDoc(tt.t, tt.vi), " "); got != tt.want {
			t.Errorf("%s: discDoc = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// typeDocs returns the doc comment text of each type declared in src.
func typeDocs(t *testing.T, src string) map[string]string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "src.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	docs := map[string]string{}
	for _, d := range file.Decls {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.TYPE {
			for _, s := range g.Specs {
				docs[s.(*ast.TypeSpec).Name.Name] = g.Doc.Text()
			}
		}
	}
	return docs
}

// TestVariantDocsNameTheDiscriminator renders a union with two variants that
// have a value, a catch-all and a key-less variant, and a union whose
// key-less variant may also be named by a present value, and requires each
// variant type's doc to keep its first line and to carry discDoc's sentence
// as its second paragraph, or no such sentence
// (0056-PLAN-bring-the-docs-current-and-keep-them-current.md, phase 4b).
func TestVariantDocsNameTheDiscriminator(t *testing.T) {
	t.Parallel()
	var schema load.Schema
	if err := json.Unmarshal([]byte(`{"$defs":{
		"U":{"description":"A union.","anyOf":[
			{"title":"a","description":"The a variant.","type":"object","required":["kind"],
				"properties":{"kind":{"type":"string","const":"a"},"x":{"type":"string"}}},
			{"title":"b","type":"object","required":["kind"],"properties":{"kind":{"type":"string","const":"b"}}},
			{"title":"other","type":"object","required":["kind"],"properties":{"kind":{"type":"string","description":"Another kind."}},
				"not":{"anyOf":[{"type":"object","required":["kind"],"properties":{"kind":{"type":"string","const":"a"}}},
					{"type":"object","required":["kind"],"properties":{"kind":{"type":"string","const":"b"}}}]}},
			{"title":"none","description":"No kind.","type":"object","properties":{"kind":{"type":"null"}}}]},
		"V":{"anyOf":[
			{"title":"c","type":"object","required":["kind"],"properties":{"kind":{"type":"string","const":"c"}}},
			{"title":"plain","type":"object","properties":{"kind":{"type":"null"}}}]}}}`), &schema); err != nil {
		t.Fatal(err)
	}
	m, err := model.Build(model.Input{Meta: &load.Meta{}, Schema: &schema})
	if err != nil {
		t.Fatal(err)
	}
	f := newFile()
	emitSchemaDef(f, m.Type("U"))
	emitSchemaDef(f, m.Type("V"))
	emitDecodeHelpers(f)
	docs := typeDocs(t, mustRender(t, f))
	const absent = `Its Kind field holds the "kind" discriminator, which the schema leaves absent or null for this variant.`
	variants := 0
	for _, u := range []*model.Type{m.Type("U"), m.Type("V")} {
		for _, vi := range u.Variants {
			variants++
			doc, ok := docs[vi.TypeName]
			if !ok {
				t.Fatalf("no type %s in the output", vi.TypeName)
			}
			paras := strings.Split(strings.TrimSpace(doc), "\n\n")
			if !strings.HasPrefix(paras[0], vi.TypeName+" is the ") {
				t.Errorf("%s: doc does not start with its name and place: %q", vi.TypeName, doc)
			}
			want := ""
			switch {
			case vi.DiscValue != "":
				want = `Its Kind field holds the "kind" discriminator, which the schema fixes at "` + vi.DiscValue + `".`
			case vi.KeyLess && u.CatchAll >= 0:
				want = absent
			}
			if want == "" {
				if strings.Contains(doc, "discriminator, which") {
					t.Errorf("%s: doc has a discriminator sentence: %q", vi.TypeName, doc)
				}
				continue
			}
			if len(paras) < 2 || strings.Join(strings.Fields(paras[1]), " ") != want {
				t.Errorf("%s: second paragraph is not %q:\n%s", vi.TypeName, want, doc)
			}
		}
	}
	if variants != 6 {
		t.Errorf("the unions have %d variants, want 6", variants)
	}
}

// TestNegotiationDocNamesItsFields requires Negotiation's doc to keep its
// first line and to name Request and Response, each as a word, so each field
// counts as documented by its type's comment
// (0056-PLAN-bring-the-docs-current-and-keep-them-current.md, phase 4b).
func TestNegotiationDocNamesItsFields(t *testing.T) {
	t.Parallel()
	doc := typeDocs(t, rendered(t, renamedProtocol(t))["types_gen.go"])["Negotiation"]
	if !strings.HasPrefix(doc, "Negotiation is ") {
		t.Errorf("Negotiation's doc does not start with its name: %q", doc)
	}
	for _, field := range []string{"Request", "Response"} {
		if !slices.Contains(strings.FieldsFunc(doc, func(r rune) bool { return !isIdentRune(r) }), field) {
			t.Errorf("Negotiation's doc does not name %s as a word: %q", field, doc)
		}
	}
}

// isIdentRune reports whether r can be part of a Go identifier.
func isIdentRune(r rune) bool {
	return r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}
