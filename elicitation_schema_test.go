package acp_test

import (
	"encoding/json"
	"errors"
	"go/build"
	"slices"
	"testing"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// problem is the field and rule of one reported problem.
type problem struct {
	Field string
	Rule  acp.ValidationRule
}

// joined returns the errors err joins, or err alone.
func joined(err error) []error {
	if err == nil {
		return nil
	}
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		return j.Unwrap()
	}
	return []error{err}
}

// problemsOf returns the field and rule of each problem err joins, read with
// errors.As, failing the test for one that is not a *ValidationError.
func problemsOf(t *testing.T, err error) []problem {
	t.Helper()
	var out []problem
	for _, e := range joined(err) {
		ve, ok := errors.AsType[*acp.ValidationError](e)
		if !ok {
			t.Fatalf("problem %v (%T) is not a *ValidationError", e, e)
		}
		out = append(out, problem{ve.Field, ve.Rule})
	}
	return out
}

// rulesWithin returns the rule of each *ValidationError err joins, and of each
// one a joined problem holds in turn, as a refused default does.
func rulesWithin(err error) []acp.ValidationRule {
	var out []acp.ValidationRule
	for _, e := range joined(err) {
		for _, x := range joined(e) {
			if ve, ok := errors.AsType[*acp.ValidationError](x); ok {
				out = append(out, ve.Rule)
			}
		}
	}
	return out
}

// TestElicitationCheckSchema reports each defect of a form once, by its field
// and rule, and a refused default also by the rule it breaks.
func TestElicitationCheckSchema(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, schema string
		want         problem
		inner        acp.ValidationRule
	}{
		{"undeclared required", `{"properties":{"a":{"type":"string"}},"required":["a","zz"]}`, problem{"required[1]", acp.RuleUndeclared}, ""},
		{"string bounds", `{"properties":{"s":{"type":"string","minLength":5,"maxLength":2}}}`, problem{`properties["s"].maxLength`, acp.RuleBounds}, ""},
		{"number bounds", `{"properties":{"n":{"type":"number","minimum":1.5,"maximum":1}}}`, problem{`properties["n"].maximum`, acp.RuleBounds}, ""},
		{"integer bounds", `{"properties":{"i":{"type":"integer","minimum":3,"maximum":2}}}`, problem{`properties["i"].maximum`, acp.RuleBounds}, ""},
		{"item bounds", `{"properties":{"a":{"type":"array","items":{"type":"string","enum":["x"]},"minItems":2,"maxItems":1}}}`, problem{`properties["a"].maxItems`, acp.RuleBounds}, ""},
		{"empty enum", `{"properties":{"s":{"type":"string","enum":[]}}}`, problem{`properties["s"].enum`, acp.RuleEnum}, ""},
		{"empty oneOf", `{"properties":{"s":{"type":"string","oneOf":[]}}}`, problem{`properties["s"].oneOf`, acp.RuleEnum}, ""},
		{"empty items enum", `{"properties":{"a":{"type":"array","items":{"type":"string","enum":[]}}}}`, problem{`properties["a"].items.enum`, acp.RuleEnum}, ""},
		{"empty items anyOf", `{"properties":{"a":{"type":"array","items":{"anyOf":[]}}}}`, problem{`properties["a"].items.anyOf`, acp.RuleEnum}, ""},
		{"duplicate enum", `{"properties":{"s":{"type":"string","enum":["x","x"]}}}`, problem{`properties["s"].enum[1]`, acp.RuleDuplicate}, ""},
		{"duplicate oneOf", `{"properties":{"s":{"type":"string","oneOf":[{"const":"x","title":"X"},{"const":"x","title":"Y"}]}}}`, problem{`properties["s"].oneOf[1].const`, acp.RuleDuplicate}, ""},
		{"duplicate items enum", `{"properties":{"a":{"type":"array","items":{"type":"string","enum":["x","y","x"]}}}}`, problem{`properties["a"].items.enum[2]`, acp.RuleDuplicate}, ""},
		{"duplicate items anyOf", `{"properties":{"a":{"type":"array","items":{"anyOf":[{"const":"x","title":"X"},{"const":"x","title":"X"}]}}}}`, problem{`properties["a"].items.anyOf[1].const`, acp.RuleDuplicate}, ""},
		{"enum and oneOf", `{"properties":{"s":{"type":"string","enum":["x"],"oneOf":[{"const":"x","title":"X"}]}}}`, problem{`properties["s"]`, acp.RuleEnum}, ""},
		{"unknown format", `{"properties":{"s":{"type":"string","format":"phone"}}}`, problem{`properties["s"].format`, acp.RuleFormat}, ""},
		{"lookahead pattern", `{"properties":{"s":{"type":"string","pattern":"(?=a)b"}}}`, problem{`properties["s"].pattern`, acp.RuleUnsupported}, ""},
		{"backreference pattern", `{"properties":{"s":{"type":"string","pattern":"(a)\\1"}}}`, problem{`properties["s"].pattern`, acp.RuleUnsupported}, ""},
		{"default too long", `{"properties":{"s":{"type":"string","maxLength":2,"default":"héllo"}}}`, problem{`properties["s"].default`, acp.RuleDefault}, acp.RuleMaxLength},
		{"default not a choice", `{"properties":{"s":{"type":"string","enum":["a"],"default":"b"}}}`, problem{`properties["s"].default`, acp.RuleDefault}, acp.RuleEnum},
		{"default not an email", `{"properties":{"s":{"type":"string","format":"email","default":"Ann <a@b.example>"}}}`, problem{`properties["s"].default`, acp.RuleDefault}, acp.RuleFormat},
		{"default not matching", `{"properties":{"s":{"type":"string","pattern":"^[a-z]+$","default":"A1"}}}`, problem{`properties["s"].default`, acp.RuleDefault}, acp.RulePattern},
		{"number default above", `{"properties":{"n":{"type":"number","maximum":1,"default":1.5}}}`, problem{`properties["n"].default`, acp.RuleDefault}, acp.RuleMaximum},
		{"integer default below", `{"properties":{"i":{"type":"integer","minimum":9007199254740993,"default":9007199254740992}}}`, problem{`properties["i"].default`, acp.RuleDefault}, acp.RuleMinimum},
		{"array default not a choice", `{"properties":{"a":{"type":"array","items":{"anyOf":[{"const":"x","title":"X"}]},"default":["y"]}}}`, problem{`properties["a"].default`, acp.RuleDefault}, acp.RuleEnum},
		{"array default too many", `{"properties":{"a":{"type":"array","items":{"type":"string","enum":["x","y"]},"maxItems":1,"default":["x","y"]}}}`, problem{`properties["a"].default`, acp.RuleDefault}, acp.RuleMaxItems},
	} {
		err := decodeSchema(t, tc.schema).CheckSchema()
		got := problemsOf(t, err)
		if len(got) != 1 || got[0] != tc.want {
			t.Errorf("%s: CheckSchema = %v (%v), want %v", tc.name, got, err, tc.want)
			continue
		}
		if tc.inner != "" {
			if rules := rulesWithin(err); !slices.Contains(rules, tc.inner) {
				t.Errorf("%s: the refused default holds the rules %v, want %s among them", tc.name, rules, tc.inner)
			}
		}
	}
}

// TestElicitationCheckSchemaValid accepts a form of every kind with every
// keyword set validly, and reports three defects in field order.
func TestElicitationCheckSchemaValid(t *testing.T) {
	t.Parallel()
	valid := decodeSchema(t, `{"type":"object","title":"Profile","required":["name","email"],"properties":{
		"name":{"type":"string","minLength":1,"maxLength":20,"pattern":"^[A-Za-z ]+$","default":"Ann"},
		"email":{"type":"string","format":"email","default":"a@b.example"},
		"site":{"type":"string","format":"uri","default":"https://example.com/a"},
		"born":{"type":"string","format":"date","default":"2024-02-29"},
		"seen":{"type":"string","format":"date-time","default":"2026-10-26t12:00:00z"},
		"pick":{"type":"string","enum":["a","b"],"default":"a"},
		"titled":{"type":"string","oneOf":[{"const":"x","title":"X"},{"const":"y","title":"Y"}],"default":"y"},
		"ratio":{"type":"number","minimum":0,"maximum":1,"default":0.5},
		"count":{"type":"integer","minimum":1,"maximum":10,"default":3},
		"ok":{"type":"boolean","default":true},
		"tags":{"type":"array","items":{"type":"string","enum":["a","b","c"]},"minItems":1,"maxItems":2,"default":["a"]},
		"labels":{"type":"array","items":{"anyOf":[{"const":"p","title":"P"}]},"default":["p"]},
		"color":{"type":"x-color","palette":"web"}}}`)
	if err := valid.CheckSchema(); err != nil {
		t.Errorf("CheckSchema of a valid form = %v, want nil", err)
	}

	three := decodeSchema(t, `{"properties":{
		"c":{"type":"string","enum":[]},
		"a":{"type":"string","minLength":5,"maxLength":2},
		"b":{"type":"string","format":"phone"}}}`)
	want := []problem{{`properties["c"].enum`, acp.RuleEnum}, {`properties["a"].maxLength`, acp.RuleBounds}, {`properties["b"].format`, acp.RuleFormat}}
	if got := problemsOf(t, three.CheckSchema()); !slices.Equal(got, want) {
		t.Errorf("CheckSchema of three defects = %v, want %v, in the form's field order", got, want)
	}

	err := decodeSchema(t, `{"properties":{"s":{"type":"string","pattern":"(?=a)b"}}}`).CheckSchema()
	if ve, ok := errors.AsType[*acp.ValidationError](err); !ok || ve.Rule != acp.RuleUnsupported || ve.Type != "ElicitationSchema" {
		t.Errorf("errors.As of an uncompilable pattern = %+v, %v; want RuleUnsupported of ElicitationSchema", ve, ok)
	}
}

// TestElicitationEmailFormat checks the email format as an RFC 5321 Mailbox,
// through a default CheckSchema refuses when it is not one.
func TestElicitationEmailFormat(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		value string
		ok    bool
	}{
		{"a@b", true},
		{"a@b.example", true},
		{"first.last+tag@sub.example.com", true},
		{"o'neil@example.com", true},
		{`"x y"@b.example`, true},
		{`"a\"b"@b.example`, true},
		{"a@[192.0.2.1]", true},
		{"a@[IPv6:2001:db8::1]", true},
		{"a@[x-tag:content]", true},
		{"Ann <a@b.example>", false},
		{" a@b.example", false},
		{"a@b.example ", false},
		{"a", false},
		{"@b.example", false},
		{"a@", false},
		{"a@b@c.example", false},
		{"a@-bad.com", false},
		{"a@bad-.com", false},
		{"a@b..example", false},
		{".a@b.example", false},
		{"a.@b.example", false},
		{"a..b@b.example", false},
		{"é@b.example", false},
		{`"unterminated@b.example`, false},
		{"a@[300.1.1.1]", false},
		{"a@[IPv6:zz]", false},
		{"a@[IPv6:fe80::1%eth0]", false},
		{"a@[2001:db8::1]", false},
	} {
		value, err := json.Marshal(tc.value)
		if err != nil {
			t.Fatal(err)
		}
		schema := decodeSchema(t, `{"properties":{"e":{"type":"string","format":"email","default":`+string(value)+`}}}`)
		if got := schema.CheckSchema() == nil; got != tc.ok {
			t.Errorf("email %q: accepted %v, want %v", tc.value, got, tc.ok)
		}
	}
}

// TestRootDoesNotImportNet keeps package acp, and so every program that
// imports it, off package net: the email format is checked by hand, not with
// net/mail (0040-PLAN-check-elicitation-forms-answers-and-urls.md, the phase 3
// deviation).
func TestRootDoesNotImportNet(t *testing.T) {
	t.Parallel()
	root, err := build.Default.ImportDir(".", 0)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var walk func(path, dir string, via []string)
	walk = func(path, dir string, via []string) {
		if seen[path] {
			return
		}
		seen[path] = true
		if path == "net" {
			t.Errorf("package acp imports net, through %v", via)
			return
		}
		pkg, err := build.Default.Import(path, dir, 0)
		if err != nil {
			// A path go/build cannot resolve here, such as a vendored one of
			// the standard library, is not net.
			return
		}
		for _, imp := range pkg.Imports {
			walk(imp, pkg.Dir, append(slices.Clone(via), imp))
		}
	}
	for _, imp := range root.Imports {
		walk(imp, root.Dir, []string{imp})
	}
}
