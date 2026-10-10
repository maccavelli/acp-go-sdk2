package acp

import (
	"encoding/json"
	"strings"
	"testing"
)

// These tests pin encoding/json v1 behavior the SDK sends on the wire, so a
// change of codec, or of Go's v1 implementation, fails here first.

// TestJSONPinEscapesHTML covers a generated union, whose own MarshalJSON
// escapes, and a plain struct, where only the caller's codec decides.
func TestJSONPinEscapesHTML(t *testing.T) {
	t.Parallel()
	const escaped = `\u003ca \u0026 b\u003e`
	for name, v := range map[string]any{
		"union":        TextBlock("<a & b>"),
		"plain struct": CancelNotification{SessionId: "<a & b>"},
	} {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), escaped) {
			t.Errorf("%s: got %s, want it to contain %s", name, b, escaped)
		}
	}
}

func TestJSONPinSortsMapKeys(t *testing.T) {
	t.Parallel()
	n := CancelNotification{
		SessionId: "s",
		Meta:      map[string]any{"h": 8, "c": 3, "f": 6, "a": 1, "g": 7, "b": 2, "e": 5, "d": 4},
	}
	b, err := json.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"_meta":{"a":1,"b":2,"c":3,"d":4,"e":5,"f":6,"g":7,"h":8},"sessionId":"s"}`
	if string(b) != want {
		t.Fatalf("got %s, want %s", b, want)
	}
}
