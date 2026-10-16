package acp

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestExamplePair builds the example agent and client and runs them as a user
// would: the client starts the agent's binary, sends its prompt, answers the
// permission request from its input, prints the stop reason and stops the
// agent. With input "1" the change is allowed and the turn ends end_turn; at
// the end of input the request is answered cancelled, and so is the turn
// (0019-PLAN-examples-docs-tests-tooling.md, E6 and E12).
func TestExamplePair(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("the go command: %v", err)
	}
	bin := t.TempDir()
	exe := func(name string) string {
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		return filepath.Join(bin, name)
	}
	agent, client := exe("agent"), exe("client")
	for _, b := range []struct{ out, pkg string }{{agent, "./example/agent"}, {client, "./example/client"}} {
		out, err := exec.CommandContext(t.Context(), gobin, "build", "-o", b.out, b.pkg).CombinedOutput()
		if err != nil {
			t.Fatalf("go build %s: %v\n%s", b.pkg, err, out)
		}
	}
	for _, tc := range []struct{ name, input, want string }{
		{"allowed", "1\n", "stop reason end_turn"},
		{"input ended", "", "stop reason cancelled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, client, agent)
			cmd.Dir = t.TempDir()
			cmd.Stdin = strings.NewReader(tc.input)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("the client: %v\n%s", err, out)
			}
			if !strings.Contains(string(out), tc.want) {
				t.Fatalf("the client's output lacks %q:\n%s", tc.want, out)
			}
		})
	}
}
