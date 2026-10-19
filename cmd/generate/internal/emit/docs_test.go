package emit

import (
	"slices"
	"strings"
	"testing"

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
		version   = "a *ProtocolVersionError when params ask for a version"
		valid     = "a *ValidationError when params break the schema"
		conn      = "a *RequestError wrapping context.Cause(ctx) when ctx ends, ErrPeerDisconnected or ErrConnectionClosed"
		answer    = "the peer's error answer as a *RequestError."
		gatedResp = "A response that breaks the schema or what initialize agreed returns an error wrapping *ValidationError or *CapabilityError"
		plainResp = "A response that breaks the schema returns an error wrapping *ValidationError,"
		undecoded = "one that does not decode a *RequestError."
		versionAn = "An answer with another protocol version returns a *ProtocolVersionError."
	)
	every := []string{notInit, capErr, termAuth, version, valid, conn, answer, gatedResp, plainResp, undecoded, versionAn}
	for _, tt := range []struct {
		name        string
		mi          model.Method
		gated, auth bool
		want        []string
	}{
		{"a gated notification", model.Method{Notif: "CancelNotification"}, true, false,
			[]string{notInit, capErr, valid, conn}},
		{"a gated request with no result", model.Method{}, true, false,
			[]string{notInit, capErr, valid, conn, answer}},
		{"a gated request", model.Method{HasResult: true}, true, false,
			[]string{notInit, capErr, valid, conn, answer, gatedResp, undecoded}},
		{"a terminal-auth request", model.Method{HasResult: true}, true, true,
			[]string{notInit, capErr, termAuth, valid, conn, answer, gatedResp, undecoded}},
		{"initialize", model.Method{HasResult: true, Initializes: true}, true, false,
			[]string{version, valid, conn, answer, gatedResp, undecoded, versionAn}},
		{"an ungated request", model.Method{HasResult: true}, false, false,
			[]string{valid, conn, answer, plainResp, undecoded}},
		{"an ungated notification", model.Method{Notif: "N"}, false, false,
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
