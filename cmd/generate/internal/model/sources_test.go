package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

// webSources are the capability rules' sources quoted from the
// specification's pages rather than the schema, with the page each is from.
var webSources = map[string]string{
	srcRootsGate:     "https://agentclientprotocol.com/protocol/session-setup",
	srcMcp:           "https://agentclientprotocol.com/protocol/session-setup",
	srcPrompt:        "https://agentclientprotocol.com/protocol/prompt-turn",
	srcAuthenticate:  "https://agentclientprotocol.com/protocol/authentication",
	srcElicitation:   "https://agentclientprotocol.com/protocol/elicitation",
	srcBooleanOption: "https://agentclientprotocol.com/protocol/session-config-options",
}

// spaces is a run of whitespace, which a schema description may break a
// sentence across.
var spaces = regexp.MustCompile(`\s+`)

// committedDescriptions returns every description in the committed schema
// files, each with its whitespace collapsed to one space. The manifest is one
// of them: a frozen rule quotes the fragment its type is built from, which
// the live unstable schema may have dropped, as schema 1.26.0 dropped NES
// (0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md, D8).
func committedDescriptions(t *testing.T) []string {
	t.Helper()
	var out []string
	var walk func(any)
	walk = func(n any) {
		switch n := n.(type) {
		case map[string]any:
			for k, v := range n {
				if s, ok := v.(string); ok && k == "description" {
					out = append(out, spaces.ReplaceAllString(s, " "))
					continue
				}
				walk(v)
			}
		case []any:
			for _, v := range n {
				walk(v)
			}
		}
	}
	for _, file := range []string{load.StableSchemaFile, load.UnstableSchemaFile, load.ManifestFile} {
		b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "schema", file))
		if err != nil {
			t.Fatal(err)
		}
		var doc any
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Fatal(err)
		}
		walk(doc)
	}
	return out
}

// TestCapabilitySourcesQuoteTheSchema requires the source of each capability
// rule of the protocol table to appear in a description of the committed
// schema, unless it is quoted from a page of the specification and listed in
// webSources, and every listed page source to be used by a rule
// (0019-PLAN-issue-fixes.md, deviation of 2026-10-18, during step 2). The
// source of each non-empty rule must appear in one too
// (0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md, D6).
func TestCapabilitySourcesQuoteTheSchema(t *testing.T) {
	t.Parallel()
	descriptions := committedDescriptions(t)
	quoted := func(source string) bool {
		quote := spaces.ReplaceAllString(source, " ")
		for _, d := range descriptions {
			if strings.Contains(d, quote) {
				return true
			}
		}
		return false
	}
	used := map[string]bool{}
	for _, r := range ACP.CapabilityRules {
		used[r.Source] = true
		if _, ok := webSources[r.Source]; ok {
			continue
		}
		if !quoted(r.Source) {
			t.Errorf("the %s rule on %s %s quotes %q, which no description of the committed schema contains",
				r.Capability, r.Definition, r.Variant, r.Source)
		}
	}
	for _, r := range ACP.NonEmptyArrays {
		if !quoted(r.Source) {
			t.Errorf("the non-empty rule on %s.%s quotes %q, which no description of the committed schema contains",
				r.Definition, r.Property, r.Source)
		}
	}
	for src, page := range webSources {
		if !used[src] {
			t.Errorf("the page source %q (%s) is used by no capability rule", src, page)
		}
	}
}
