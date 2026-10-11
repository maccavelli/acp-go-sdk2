package load

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadErrors requires each reader to name its file when the file cannot
// be read, here because a directory stands in its place. Parse errors and
// missing files are engine cases (testdata/engine/refuse-*.txtar).
func TestReadErrors(t *testing.T) {
	t.Parallel()
	read := map[string]func(dir string) error{
		"meta.json":            func(dir string) error { _, err := ReadMeta(dir); return err },
		"schema.json":          func(dir string) error { _, err := ReadSchema(dir); return err },
		"meta.unstable.json":   func(dir string) error { _, _, err := ReadMetaUnstable(dir); return err },
		"schema.unstable.json": func(dir string) error { _, _, err := ReadSchemaUnstable(dir); return err },
	}
	for name, fn := range read {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, name), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := fn(dir); err == nil || !strings.Contains(err.Error(), "read "+name) {
				t.Errorf("error = %v, want one starting read %s", err, name)
			}
		})
	}
}

// TestMergeRefusesNilInputs requires the merge to refuse each missing input
// by name; the loader never passes one, so only a caller in code can.
func TestMergeRefusesNilInputs(t *testing.T) {
	t.Parallel()
	m, s := &Meta{}, &Schema{}
	tests := []struct {
		name string
		sm   *Meta
		ss   *Schema
		um   *Meta
		us   *Schema
		want string
	}{
		{"stable meta", nil, s, m, s, "stable meta is nil"},
		{"stable schema", m, nil, m, s, "stable schema is nil"},
		{"unstable meta", m, s, nil, s, "unstable meta is nil"},
		{"unstable schema", m, s, m, nil, "unstable schema is nil"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, _, _, err := MergeStableAndUnstable(tt.sm, tt.ss, tt.um, tt.us); err == nil || err.Error() != tt.want {
				t.Errorf("error = %v, want %q", err, tt.want)
			}
		})
	}
	// buildUnstableDuplicateSet checks its schemas again; MergeStableAndUnstable
	// has refused nil ones before it calls it.
	if _, err := buildUnstableDuplicateSet(nil, s, nil); err == nil || err.Error() != "stable schema is nil" {
		t.Errorf("buildUnstableDuplicateSet(nil stable) error = %v", err)
	}
	if _, err := buildUnstableDuplicateSet(s, nil, nil); err == nil || err.Error() != "unstable schema is nil" {
		t.Errorf("buildUnstableDuplicateSet(nil unstable) error = %v", err)
	}
}
