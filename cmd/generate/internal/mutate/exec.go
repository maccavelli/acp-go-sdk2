package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The outcomes of a mutant.
const (
	killed   = "killed"
	survived = "survived"
	invalid  = "invalid"
	timedOut = "timeout"
)

// result is the record of one mutant in the report.
type result struct {
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Column   int      `json:"column"`
	Operator string   `json:"operator"`
	Original string   `json:"original"`
	Mutant   string   `json:"mutant"`
	Outcome  string   `json:"outcome"`
	Failed   []string `json:"failed,omitempty"`
	Seconds  float64  `json:"seconds"`
	// Error is the go command's output when the mutant did not build.
	Error string `json:"error,omitempty"`
}

// testEvent is the part of a go test -json event the runner reads.
type testEvent struct {
	Action      string
	Test        string
	Output      string
	FailedBuild string
}

// grace is how long the go command may run beyond the test timeout, for
// building the mutant and reporting.
const grace = 2 * time.Minute

// runMutant writes the mutant of s into work, and runs go test in dir with an
// overlay that puts it in place of the file, so the tree is never written.
func runMutant(ctx context.Context, cfg config, work string, n int, s site, src []byte) (r result) {
	r = result{File: s.File, Line: s.Line, Column: s.Col, Operator: s.Op, Original: s.Orig, Mutant: s.Repl}
	start := time.Now()
	defer func() { r.Seconds = time.Since(start).Round(10 * time.Millisecond).Seconds() }()
	out, err := mutant(s.File, src, s)
	if err != nil {
		r.Outcome, r.Error = invalid, err.Error()
		return r
	}
	file := filepath.Join(work, strconv.Itoa(n)+".go")
	overlay := filepath.Join(work, strconv.Itoa(n)+".json")
	abs, err := filepath.Abs(filepath.Join(cfg.dir, filepath.FromSlash(s.File)))
	if err == nil {
		err = os.WriteFile(file, out, 0o600)
	}
	if err == nil {
		var ov []byte
		ov, err = json.Marshal(map[string]map[string]string{"Replace": {abs: file}})
		if err == nil {
			err = os.WriteFile(overlay, ov, 0o600)
		}
	}
	if err != nil {
		r.Outcome, r.Error = invalid, err.Error()
		return r
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.wait+grace)
	defer cancel()
	args := []string{"test", "-overlay=" + overlay, "-count=1", "-failfast", "-json", "-timeout=" + cfg.timeout}
	if cfg.skip != "" {
		args = append(args, "-skip="+cfg.skip)
	}
	args = append(args, strings.Fields(cfg.pkgs)...)
	cmd := exec.CommandContext(ctx, "go", args...) //nolint:gosec // G204: the go command, with the tool's own flags.
	cmd.Dir = cfg.dir
	cmd.WaitDelay = 10 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	classify(&r, runErr, ctx.Err(), &stdout, stderr.String())
	return r
}

// classify sets r's outcome from the go command's exit and its -json events:
// exit 0 is survived; a build failure is invalid; a test timeout, or the
// command outliving its deadline, is timeout; a failed test is killed.
func classify(r *result, runErr, ctxErr error, stdout io.Reader, stderr string) {
	if runErr == nil {
		r.Outcome = survived
		return
	}
	if errors.Is(ctxErr, context.DeadlineExceeded) {
		r.Outcome = timedOut
		return
	}
	var built, failed, timeout bool
	var buildOut strings.Builder
	dec := json.NewDecoder(stdout)
	for {
		var e testEvent
		if dec.Decode(&e) != nil {
			break
		}
		switch {
		case e.Action == "build-output":
			buildOut.WriteString(e.Output)
		case e.Action == "build-fail" || e.FailedBuild != "":
			built = true
		case e.Action == "fail":
			failed = true
			if e.Test != "" && !slices.Contains(r.Failed, e.Test) {
				r.Failed = append(r.Failed, e.Test)
			}
		case strings.Contains(e.Output, "panic: test timed out after"):
			timeout = true
		}
	}
	slices.Sort(r.Failed)
	switch {
	case built || !failed:
		r.Outcome, r.Error = invalid, strings.TrimSpace(buildOut.String()+stderr)
		r.Failed = nil
	case timeout:
		r.Outcome, r.Failed = timedOut, nil
	default:
		r.Outcome = killed
	}
}
