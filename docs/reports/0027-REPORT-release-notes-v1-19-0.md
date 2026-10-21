# Release notes for v1.19.0, against v1.18.0

Observed 2026-10-21 on `084ec9b`, for [0027-PLAN-agent-process-helper.md](../decisions/0027-PLAN-agent-process-helper.md), step 7. This
report drafts the text of the GitHub release for `v1.19.0`, as `RELEASING.md` describes it. It decides nothing.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `084ec9b` with nothing uncommitted.

Bring it up to date if more changes land before the tag.

- Four additions to the exported API, all in the new package `agentproc`; the root package's exported API and its code are unchanged.
  `make api` against `v1.18.0`: "0 incompatible, 4 compatible". The release is proposed as the minor `v1.19.0`, as `RELEASING.md` says for
  compatible changes. The owner confirms it at the tag.
- What a consumer may meet: the `go` line is now `1.26.10`, so a consumer whose own is lower has it raised when it requires this release;
  and the module requires `golang.org/x/sys`, which is built only for `agentproc` on Windows.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.19.0

The ACP schema is still `1.25.0` (`git diff v1.18.0 -- schema/` is empty), and the root package `acp` is unchanged. This release adds
[0027-MADR-agent-process-helper.md](../decisions/0027-MADR-agent-process-helper.md)'s package `agentproc`, which starts an ACP agent as a
child process and stops it, with the processes it started, in bounded steps; and raises the Go floor to 1.26.10
([0028-MADR-pin-go-1-27-3-and-floor-1-26-10.md](../decisions/0028-MADR-pin-go-1-27-3-and-floor-1-26-10.md)). `make api` reports four
compatible additions and no incompatible change.

#### New: `github.com/maccavelli/acp-go-sdk2/agentproc`

- **`Command{Cmd, Grace, ConnectionOptions}` and its `Start(client)`,** which returns a `*Process` with `Conn`, `Done`, `Wait` and
  `Close`; and the errors `ErrTerminated` and `ErrKilled`. (`e33eec2`, `529bdac`, `449b2b6`.)
- **`Start`** connects a `ClientSideConnection` over the agent's standard input and output, through pipes it owns, so nothing the agent
  writes before it exits is lost. The `*exec.Cmd` is the caller's: its `Path`, `Args`, `Env`, `Dir` and `Stderr` stay as set. `Start`
  refuses, starting nothing, a nil `Cmd` or client, a `Cmd` already started, or one whose `Stdin` or `Stdout` is set; and sets a zero
  `WaitDelay` to half the grace.
- **The agent's processes end with it.** On Unix the agent runs in a process group of its own; on Windows in a job object that also ends
  the agent's processes should the client die; elsewhere only the agent is ended.
- **`Close`** closes the agent's input and waits up to the grace, 5 seconds unless `Grace` says otherwise; on Unix it then sends SIGTERM
  to the group and waits again; then it kills the group, or ends the job, and closes the connection. `Close` and `Wait` return the
  agent's exit error, joined with `ErrTerminated` or `ErrKilled` when `Close` had to escalate. A second `Close` returns the same error.
- **The process and the connection are tied:** when the agent exits, what is left of its group or job is killed, and the connection is
  closed once it has read to the end of the agent's output; when the connection ends first, the agent is stopped.

```go
ctx := context.Background()
cmd := exec.Command("my-agent", "--acp")
// The agent's log.
cmd.Stderr = os.Stderr
p, err := (&agentproc.Command{Cmd: cmd}).Start(client{})
if err != nil {
    log.Fatal(err)
}
defer func() {
    if err := p.Close(); err != nil {
        log.Printf("agent: %v", err)
    }
}()
if _, err := p.Conn().Initialize(ctx, acp.InitializeRequest{}); err != nil {
    log.Printf("initialize: %v", err)
}
```

#### Requirements

- **Go 1.26.10 or newer,** the `go` line of `go.mod`, which was `1.26.9`. Go 1.26.10 and 1.27.3, both released 2026-10-19, carry security
  fixes to `crypto/x509`; no package of this module imports it. A consumer whose `go` line is lower has it raised when it requires this
  release. (`0665658`.)
- **`golang.org/x/sys v0.49.0`,** the module's first requirement, for `agentproc`'s job object on Windows; no other package or platform
  builds it. (`449b2b6`.)

#### Examples

- `example/client`, `example/claude-code` and `example/gemini` start and stop their agent through `agentproc`, in place of the
  hand-written `stopAgent` each carried. (`432426d`.)

#### Documentation

- The README's "Closing and limits" describes `agentproc`, with the example above, and `docs/architecture.md` its design. (`084ec9b`.)

#### For maintainers

- **The toolchain is go1.27.3,** in both `toolchain` lines and CI's newest job; CI's floor job runs 1.26.10. A new CI step, "the newest is
  the toolchain line", fails when the newest job's version and a `toolchain` line disagree. (`0665658`.)
- **The API gate covers `agentproc`.** `scripts/check_api.py` compares the root package and `agentproc` with the newest `v1` tag; a
  package missing from the tag is compared against an empty one. (`e33eec2`.)
- **New tests:** `agentproc`'s, which run the test binary itself as the agent, under its own goroutine-leak guard, on Unix and on Windows;
  and at the root, `TestReadmeAgentprocIsTheExample`. (`e33eec2`, `529bdac`, `449b2b6`, `084ec9b`.)

## Sources not in the release text

- **Commits since `v1.18.0`** (`git log v1.18.0..084ec9b`), thirteen:
  - `e33eec2` adds `agentproc` (`agentproc.go`, `tree_unix.go`, `tree_other.go`, the tests) and changes `scripts/check_api.py`;
  - `529bdac` ties the process to the connection, in `agentproc`;
  - `449b2b6` adds `tree_windows.go` and its tests, and `golang.org/x/sys` to `go.mod` and `go.sum`;
  - `432426d` changes the three examples;
  - `0665658` changes both `go.mod` files, `ci.yml`, `README.md` and a generator test's fixtures;
  - `084ec9b` changes `README.md`, `RELEASING.md`, `docs/architecture.md`, `docs_test.go` and `agentproc`'s tests;
  - `d034801`, `1ea8475`, `68130ca`, `5930996`, `cfc3d12`, `ed150be` and `2bd014f` change records only (`git show --name-only`).
- **Go files changed.** `git diff --name-only v1.18.0 084ec9b -- '*.go' ':!*_test.go'` lists seven: `agentproc`'s four and the three
  examples' `main.go`. No file of the root package but a test changed.
- **`make api` on `084ec9b`:** exit 0, "against v1.18.0: 0 incompatible, 4 compatible": `agentproc: Command`, `agentproc: ErrKilled`,
  `agentproc: ErrTerminated` and `agentproc: Process`, each "added".
- **The evidence:** [0027-PLAN-agent-process-helper.md](../decisions/0027-PLAN-agent-process-helper.md), its steps 1 to 6, with their
  planted breaks, gates on three platforms and deviations (the `sync.Once` plant that cannot fail; `WaitDelay` half the grace), and the
  correction of the toolchain its steps 2 to 5 ran on; and
  [0028-PLAN-pin-go-1-27-3-and-floor-1-26-10.md](../decisions/0028-PLAN-pin-go-1-27-3-and-floor-1-26-10.md), with CI run `39622421451`.
- **Not in this release:** schema `1.26.0`, deferred by the owner
  ([0025-REPORT-schema-1-26-0-adoption-assessment.md](0025-REPORT-schema-1-26-0-adoption-assessment.md)); `Pdeathsig` on Linux; anything on
  the agent side.
