package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
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
// files, each with its whitespace collapsed to one space.
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
	for _, file := range []string{"schema.json", "schema.unstable.json"} {
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
// (0019-PLAN-issue-fixes.md, deviation of 2026-10-18, during step 2).
func TestCapabilitySourcesQuoteTheSchema(t *testing.T) {
	t.Parallel()
	descriptions := committedDescriptions(t)
	used := map[string]bool{}
	for _, r := range ACP.CapabilityRules {
		used[r.Source] = true
		if _, ok := webSources[r.Source]; ok {
			continue
		}
		quote := spaces.ReplaceAllString(r.Source, " ")
		found := false
		for _, d := range descriptions {
			if strings.Contains(d, quote) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("the %s rule on %s %s quotes %q, which no description of the committed schema contains",
				r.Capability, r.Definition, r.Variant, r.Source)
		}
	}
	for src, page := range webSources {
		if !used[src] {
			t.Errorf("the page source %q (%s) is used by no capability rule", src, page)
		}
	}
}
