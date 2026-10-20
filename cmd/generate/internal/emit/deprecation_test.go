package emit

import (
	"slices"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// TestWithNotice requires the doc unchanged when nothing marks the node;
// otherwise the notice as the doc's last paragraph, after a blank line, or
// alone, its note after the sentence, wrapped at 76 characters; and the
// doc it was given unchanged (0024-PLAN-api-gate-and-deprecation.md).
func TestWithNotice(t *testing.T) {
	t.Parallel()
	doc := []string{"Old is a type.", "", "It is old."}
	if got := withNotice(doc, "the Old schema definition", nil); !slices.Equal(got, doc) {
		t.Errorf("no mark: %q, want the doc", got)
	}
	got := withNotice(doc, "the Old schema definition", &model.Deprecation{})
	want := []string{"Old is a type.", "", "It is old.", "", "Deprecated: the ACP schema marks the Old schema definition deprecated."}
	if !slices.Equal(got, want) {
		t.Errorf("a mark: %q, want %q", got, want)
	}
	if !slices.Equal(doc, []string{"Old is a type.", "", "It is old."}) {
		t.Errorf("the doc given was changed: %q", doc)
	}
	// A doc with room to grow is shared by its callers, such as a variant's
	// field and its inline type: one notice must not overwrite another.
	spare := append(make([]string, 0, 8), "A doc.")
	first := withNotice(spare, "the A schema definition", &model.Deprecation{})
	withNotice(spare, "the B schema definition", &model.Deprecation{})
	if want := []string{"A doc.", "", "Deprecated: the ACP schema marks the A schema definition deprecated."}; !slices.Equal(first, want) {
		t.Errorf("the doc given was changed: the first notice is now %q", first)
	}
	got = withNotice(nil, `the "a" property of the Holder schema definition`, &model.Deprecation{Note: "Use b, which replaces it."})
	want = []string{
		`Deprecated: the ACP schema marks the "a" property of the Holder schema`,
		"definition deprecated. Use b, which replaces it.",
	}
	if !slices.Equal(got, want) {
		t.Errorf("a mark with a note, no doc: %q, want %q", got, want)
	}
	for _, line := range got {
		if len(line) > 76 {
			t.Errorf("line of %d characters: %q", len(line), line)
		}
	}
	mi := &model.Method{Deprecation: &model.Deprecation{Definition: "InitRequest"}}
	if got := methodNotice(nil, mi); !slices.Equal(got, []string{"Deprecated: the ACP schema marks the InitRequest schema definition", "deprecated."}) {
		t.Errorf("a method's notice: %q", got)
	}
	if got := methodNotice(doc, &model.Method{}); !slices.Equal(got, doc) {
		t.Errorf("an unmarked method: %q, want the doc", got)
	}
}
