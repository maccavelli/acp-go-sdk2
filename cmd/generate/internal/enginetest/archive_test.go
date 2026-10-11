package enginetest

import (
	"strings"
	"testing"
)

func TestParseArchive(t *testing.T) {
	t.Parallel()
	a, err := ParseArchive([]byte("what the case pins\n\n-- a.json --\n{}\n-- b.txt --\none\ntwo"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(a.Comment); got != "what the case pins\n\n" {
		t.Errorf("comment = %q", got)
	}
	if len(a.Files) != 2 {
		t.Fatalf("files = %d, want 2", len(a.Files))
	}
	if a.Files[0].Name != "a.json" || string(a.Files[0].Data) != "{}\n" {
		t.Errorf("first file = %q %q", a.Files[0].Name, a.Files[0].Data)
	}
	// The last file's data ends with a newline even when the archive does not.
	if b, ok := a.File("b.txt"); !ok || string(b) != "one\ntwo\n" {
		t.Errorf("b.txt = %q, %v", b, ok)
	}
	if _, ok := a.File("c.txt"); ok {
		t.Error("File found c.txt, which the archive does not hold")
	}
}

func TestParseArchiveWithoutFiles(t *testing.T) {
	t.Parallel()
	a, err := ParseArchive([]byte("only a comment\n"))
	if err != nil {
		t.Fatal(err)
	}
	if string(a.Comment) != "only a comment\n" || len(a.Files) != 0 {
		t.Errorf("got comment %q and %d files", a.Comment, len(a.Files))
	}
}

func TestParseArchiveRefuses(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, in, want string }{
		{"duplicate name", "-- a.json --\n{}\n-- a.json --\n[]\n", `duplicate file "a.json"`},
		{"parent directory", "-- ../x.json --\n{}\n", `file name "../x.json"`},
		{"subdirectory", "-- d/x.json --\n{}\n", `file name "d/x.json"`},
		{"backslash", "-- d\\x.json --\n{}\n", `file name "d\\x.json"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseArchive([]byte(tt.in))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want one containing %s", err, tt.want)
			}
		})
	}
}

func TestFormatArchive(t *testing.T) {
	t.Parallel()
	in := "comment\n-- a.json --\n{}\n-- b.txt --\nx\n"
	a, err := ParseArchive([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(Format(a)); got != in {
		t.Errorf("Format = %q, want %q", got, in)
	}
	a.Set("b.txt", []byte("y\n"))
	a.Set("c.txt", []byte("z"))
	if got, want := string(Format(a)), "comment\n-- a.json --\n{}\n-- b.txt --\ny\n-- c.txt --\nz\n"; got != want {
		t.Errorf("Format after Set = %q, want %q", got, want)
	}
}
