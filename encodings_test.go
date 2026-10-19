package acp

import (
	"encoding/json"
	"flag"
	"os"
	"slices"
	"strings"
	"testing"
)

// update rewrites testdata/encodings.golden from the tree, for a change to
// the encodings that is meant (0019-PLAN-idioms-performance.md, D14).
var update = flag.Bool("update", false, "rewrite testdata/encodings.golden from the tree")

// encodingsGolden holds one line per schema instance of every registered
// type: the type, a tab, and the instance's encoding after a decode.
const encodingsGolden = "testdata/encodings.golden"

// TestEncodingsUnchanged requires the encoding of every schema instance of
// every registered type to equal its line in testdata/encodings.golden, byte
// for byte, so a change to the generated codecs cannot change the wire
// format unseen. An instance that does not decode or encode keeps its error
// there instead. The host's temporary directory, which the instances use as
// an absolute path, is written as <instancePath>, so the file is the same on
// every host (0019-PLAN-idioms-performance.md, D14).
func TestEncodingsUnchanged(t *testing.T) {
	t.Parallel()
	got := encodings(t)
	if *update {
		if err := os.WriteFile(encodingsGolden, got, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(encodingsGolden)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == string(want) {
		return
	}
	gl, wl := strings.Split(string(got), "\n"), strings.Split(string(want), "\n")
	differ := 0
	for i := range max(len(gl), len(wl)) {
		var g, w string
		if i < len(gl) {
			g = gl[i]
		}
		if i < len(wl) {
			w = wl[i]
		}
		if g == w {
			continue
		}
		if differ == 0 {
			t.Errorf("%s line %d:\n got: %s\nwant: %s", encodingsGolden, i+1, g, w)
		}
		differ++
	}
	t.Errorf("%d lines differ from %s; rerun with -update if the change is meant", differ, encodingsGolden)
}

// encodings returns the sorted lines of every schema instance's encoding.
func encodings(t *testing.T) []byte {
	t.Helper()
	quoted, err := json.Marshal(instancePath)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, st := range schemaTypes() {
		for _, inst := range newBuilder(st.File).instances(st.Def) {
			raw, err := json.Marshal(inst)
			if err != nil {
				t.Fatal(err)
			}
			line := st.Type + "\t"
			v := st.New()
			if err := json.Unmarshal(raw, v); err != nil {
				line += "decode error: " + err.Error()
			} else if out, err := json.Marshal(v); err != nil {
				line += "encode error: " + err.Error()
			} else {
				line += string(out)
			}
			line = strings.ReplaceAll(line, string(quoted), `"<instancePath>"`)
			lines = append(lines, strings.ReplaceAll(line, instancePath, "<instancePath>"))
		}
	}
	slices.Sort(lines)
	return []byte(strings.Join(lines, "\n") + "\n")
}
