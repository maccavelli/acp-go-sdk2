package acp_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// checkFixture is one file of testdata/elicitation/check: a form, and answers
// to it with the problems Check must report, in order.
type checkFixture struct {
	Description     string          `json:"description"`
	RequestedSchema json.RawMessage `json:"requestedSchema"`
	Answers         []struct {
		Name    string          `json:"name"`
		Content json.RawMessage `json:"content"`
		Want    []struct {
			Field string `json:"field"`
			Rule  string `json:"rule"`
		} `json:"want"`
	} `json:"answers"`
}

// answerTexts returns the strings an answer value holds, which a message must
// not quote; short ones are left out, since they occur in any message.
func answerTexts(v acp.ElicitationContentValue) []string {
	var out []string
	switch {
	case v.String != nil:
		out = append(out, string(*v.String))
	case v.StringArray != nil:
		out = append(out, *v.StringArray...)
	}
	return slices.DeleteFunc(out, func(s string) bool { return len(strings.TrimSpace(s)) < 4 })
}

// TestElicitationCheckFixtures checks each answer of each fixture, decoded
// from its JSON as the wire decodes it, and requires the problems in order;
// no problem's message holds the value it is about.
func TestElicitationCheckFixtures(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("testdata", "elicitation", "check", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("fixtures: %v, %v", files, err)
	}
	answers := 0
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var fx checkFixture
		if err := json.Unmarshal(b, &fx); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		schema := decodeSchema(t, string(fx.RequestedSchema))
		for _, a := range fx.Answers {
			answers++
			var content map[string]acp.ElicitationContentValue
			if err := json.Unmarshal(a.Content, &content); err != nil {
				t.Fatalf("%s, %s: content: %v", file, a.Name, err)
			}
			err := schema.Check(content)
			got := problemsOf(t, err)
			var want []problem
			for _, w := range a.Want {
				want = append(want, problem{fmt.Sprintf("content[%q]", w.Field), acp.ValidationRule(w.Rule)})
			}
			if !slices.Equal(got, want) {
				t.Errorf("%s, %s: Check = %v, want %v (%v)", filepath.Base(file), a.Name, got, want, err)
			}
			for _, e := range joined(err) {
				ve, _ := errors.AsType[*acp.ValidationError](e)
				if ve.Type != "CreateElicitationAccept" {
					t.Errorf("%s, %s: a problem of Type %q, want CreateElicitationAccept", filepath.Base(file), a.Name, ve.Type)
				}
				v, _ := ve.Value.(acp.ElicitationContentValue)
				for _, text := range answerTexts(v) {
					if strings.Contains(ve.Error(), text) {
						t.Errorf("%s, %s: the message %q holds the answer's value", filepath.Base(file), a.Name, ve.Error())
					}
				}
			}
		}
	}
	if answers < 50 {
		t.Errorf("the fixtures hold %d answers, want at least 50", answers)
	}
}

// TestElicitationParseValue converts a text interface's input per kind,
// losslessly, never trimming, and refuses what it cannot convert, with a
// message that holds no text.
func TestElicitationParseValue(t *testing.T) {
	t.Parallel()
	str, num, integer := acp.NewElicitationPropertySchemaString(), acp.NewElicitationPropertySchemaNumber(), acp.NewElicitationPropertySchemaInteger()
	boolean, array := acp.NewElicitationPropertySchemaBoolean(), acp.NewElicitationPropertySchemaArray(acp.MultiSelectItems{})
	for _, tc := range []struct {
		name string
		p    acp.ElicitationPropertySchema
		text []string
		want string
	}{
		{"a string, as it is", str, []string{"  Ann  "}, `"  Ann  "`},
		{"a number", num, []string{"0.25"}, "0.25"},
		{"a whole number", num, []string{"5"}, "5"},
		{"an integer", integer, []string{"9007199254740993"}, "9007199254740993"},
		{"a negative integer", integer, []string{"-42"}, "-42"},
		{"a boolean", boolean, []string{"false"}, "false"},
		{"a list", array, []string{"a", "b"}, `["a","b"]`},
		{"an empty list", array, nil, "[]"},
	} {
		v, err := tc.p.ParseValue(tc.text...)
		if err != nil {
			t.Errorf("%s: ParseValue(%q) = %v", tc.name, tc.text, err)
			continue
		}
		if b, err := json.Marshal(v); err != nil || string(b) != tc.want {
			t.Errorf("%s: ParseValue(%q) = %s, %v; want %s", tc.name, tc.text, b, err, tc.want)
		}
	}
	for _, tc := range []struct {
		name string
		p    acp.ElicitationPropertySchema
		text []string
	}{
		{"a fraction for an integer", integer, []string{"3.7"}},
		{"an exponent for an integer", integer, []string{"1e20"}},
		{"an integer out of range", integer, []string{"9223372036854775808"}},
		{"a space around an integer", integer, []string{" 5"}},
		{"NaN", num, []string{"NaN"}},
		{"Inf", num, []string{"Inf"}},
		{"text for a number", num, []string{"five"}},
		{"yes for a boolean", boolean, []string{"yes"}},
		{"two texts for a string", str, []string{"a", "b"}},
		{"no text for a string", str, nil},
		{"a type it does not know", acp.NewElicitationPropertySchemaOther("x-color"), []string{"teal"}},
	} {
		_, err := tc.p.ParseValue(tc.text...)
		ve, ok := errors.AsType[*acp.ValidationError](err)
		if !ok || ve.Rule != acp.RuleType {
			t.Errorf("%s: ParseValue(%q) = %v, want a RuleType *ValidationError", tc.name, tc.text, err)
			continue
		}
		for _, text := range tc.text {
			if len(text) > 3 && strings.Contains(ve.Error(), text) {
				t.Errorf("%s: the message %q holds the text", tc.name, ve.Error())
			}
		}
	}
}

// TestElicitationContentNumbers reads a numeric answer whichever variant the
// client's encoder chose.
func TestElicitationContentNumbers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw string
		f   float64
		fOK bool
		n   int64
		nOK bool
	}{
		{"5", 5, true, 5, true},
		{"5.0", 5, true, 5, true},
		{"5.5", 5.5, true, 0, false},
		{"9007199254740993", 9007199254740992, true, 9007199254740993, true},
		{"1e20", 1e20, true, 0, false},
		{`"5"`, 0, false, 0, false},
		{"true", 0, false, 0, false},
	} {
		var v acp.ElicitationContentValue
		if err := json.Unmarshal([]byte(tc.raw), &v); err != nil {
			t.Fatal(err)
		}
		if f, ok := v.Float64(); ok != tc.fOK || f != tc.f {
			t.Errorf("Float64 of %s = %v, %v; want %v, %v", tc.raw, f, ok, tc.f, tc.fOK)
		}
		if n, ok := v.Int64(); ok != tc.nOK || n != tc.n {
			t.Errorf("Int64 of %s = %d, %v; want %d, %v", tc.raw, n, ok, tc.n, tc.nOK)
		}
	}
	if _, ok := acp.NewElicitationContentValueNumber(math.NaN()).Int64(); ok {
		t.Error("Int64 of NaN is true")
	}
}
