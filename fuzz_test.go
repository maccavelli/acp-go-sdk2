package acp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goldenTypes names the type of each file of testdata/json_golden by its
// prefix or full name, so FuzzRoundTrip seeds each file with its own type.
var goldenTypes = []struct{ prefix, typ string }{
	{"content_", "ContentBlock"},
	{"tool_content_", "ToolCallContent"},
	{"permission_outcome_", "RequestPermissionOutcome"},
	{"session_update_", "SessionUpdate"},
	{"cancel_notification", "CancelNotification"},
	{"fs_read_text_file_request", "ReadTextFileRequest"},
	{"fs_read_text_file_response", "ReadTextFileResponse"},
	{"fs_write_text_file_request", "WriteTextFileRequest"},
	{"initialize_request", "InitializeRequest"},
	{"initialize_response", "InitializeResponse"},
	{"new_session_request", "NewSessionRequest"},
	{"new_session_response", "NewSessionResponse"},
	{"prompt_request", "PromptRequest"},
	{"request_permission_request", "RequestPermissionRequest"},
	{"request_permission_response_", "RequestPermissionResponse"},
}

// FuzzRoundTrip requires a value that a generated type decodes to encode, to
// decode again from that encoding, and to encode the second time to the same
// bytes; Validate, where the type has it, must not panic
// (0011-PLAN-engine-tests.md, D4). JSON null, which decodes to no value, is
// skipped. which picks the type of schema_gen_test.go.
// The seeds are every file of testdata/json_golden with its type, and every
// instance the conformance test builds.
func FuzzRoundTrip(f *testing.F) {
	types := schemaTypes()
	index := map[string]uint16{}
	for i, st := range types {
		index[st.Type] = uint16(i)
	}
	golden, err := filepath.Glob(filepath.Join("testdata", "json_golden", "*.json"))
	if err != nil || len(golden) == 0 {
		f.Fatalf("no golden files: %v", err)
	}
	for _, p := range golden {
		typ := goldenType(filepath.Base(p))
		i, ok := index[typ]
		if !ok {
			f.Fatalf("%s: no type in goldenTypes, or %q is not in schemaTypes", p, typ)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(i, b)
	}
	for i, st := range types {
		for _, inst := range newBuilder(st.File).instances(st.Def) {
			b, err := json.Marshal(inst)
			if err != nil {
				f.Fatal(err)
			}
			f.Add(uint16(i), b)
		}
	}
	f.Fuzz(func(t *testing.T, which uint16, data []byte) {
		st := types[int(which)%len(types)]
		v := st.New()
		// JSON null decodes to no value: a union it leaves unset does not
		// encode (README, "Decoding JSON null into a union").
		if json.Unmarshal(data, v) != nil || string(bytes.TrimSpace(data)) == "null" {
			return
		}
		if val, ok := v.(interface{ Validate() error }); ok {
			_ = val.Validate()
		}
		first, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("%s: %s decoded but does not encode: %v", st.Type, data, err)
		}
		again := st.New()
		if err := json.Unmarshal(first, again); err != nil {
			t.Fatalf("%s: %s decoded, but its encoding %s does not: %v", st.Type, data, first, err)
		}
		second, err := json.Marshal(again)
		if err != nil {
			t.Fatalf("%s: %s, decoded again, does not encode: %v", st.Type, first, err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("%s: %s encodes as %s, and that as %s", st.Type, data, first, second)
		}
	})
}

// goldenType returns the type goldenTypes gives the golden file name, or "".
func goldenType(name string) string {
	name = strings.TrimSuffix(name, ".json")
	for _, g := range goldenTypes {
		if name == g.prefix || strings.HasSuffix(g.prefix, "_") && strings.HasPrefix(name, g.prefix) {
			return g.typ
		}
	}
	return ""
}

// FuzzCanonicalIDKey requires canonicalJSONRPCIDKey, with its fast path for a
// plain integer, to give every id the key and the error the JSON decoder gives
// it, called directly (0019-PLAN-idioms-performance.md, D5). The seeds are the
// ids the plan measured, the fast path's edges, and ids it must leave to the
// decoder.
func FuzzCanonicalIDKey(f *testing.F) {
	for _, seed := range []string{
		"0", "12345", "18446744073709551615", "-0", "1e2", "1.0", "007",
		"9", "1234567890123456789", "12345678901234567890", "00", "01", " 42 ", "\t7\n",
		"+1", "0x1", "1 2", `"x"`, `"12"`, "null", "", "[1]",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		got, gotErr := canonicalJSONRPCIDKey(raw)
		want, wantErr := decodeJSONRPCIDKey(bytes.TrimSpace(raw))
		if got != want || (gotErr == nil) != (wantErr == nil) || (gotErr != nil && gotErr.Error() != wantErr.Error()) {
			t.Errorf("id %q: key %q, error %v; the decoder gives %q, %v", raw, got, gotErr, want, wantErr)
		}
	})
}
