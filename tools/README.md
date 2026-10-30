# Verification tools

What a PLAN's steps run before their record, beyond `make pre-add-check`
([0031-MADR-verification-tools-in-repository.md](../docs/decisions/0031-MADR-verification-tools-in-repository.md)).
Python 3 and its standard library; each tool is run as `python3 tools/<tool>.py`
and prints its usage with `--help`.

## Setting up

| Variable | For | Value |
| :--- | :--- | :--- |
| `ACP_LINUX_HOST` | `remote.py --host linux`, the `linux` gate | an SSH host or alias with Go |
| `ACP_WINDOWS_HOST` | `remote.py --host windows`, the `windows` gate, `leftovers.py` | an SSH host or alias with Go and Git's bash |
| `DISCLOSURE_DENY` | `idscan.py` | the push guard's deny list; unset, `~/.config/git/disclosure-deny` |
| `ACP_TOOLS_CACHE` | every tool's cache | unset, the user's cache directory, under `acp-go-sdk2-tools` |

No host name, account or path of a machine belongs in this directory: the
hosts come from the variables, and the identifiers from the deny list. A tool
that needs a variable that is unset stops and names it.

Logs go to `.cache/tools/<tool>/`, which git ignores; `--out DIR` puts them
elsewhere. Caches, Go's and the planted breaks' copies of the tree, go to the
cache directory, outside the tree.

## The tools

| Tool | What it does |
| :--- | :--- |
| `gates.py [--all] [GATE...]` | the gate set, one gate at a time; no gate named, every gate but the hosts' |
| `floor.py` | `go vet` and `go test -race` in both modules at the `go` line's toolchain |
| `platforms.py` | `golangci-lint` for Linux and FreeBSD, and `go vet` per platform, over `./agentproc/...`, `./clienthost/...` and `./internal/...` |
| `remote.py tree --host H` | the tree's tests on a test host |
| `remote.py pkg --host H PKG [FLAG...]` | one package's test binary, built here, run there with the flags |
| `leftovers.py` | test binaries left running on the Windows host |
| `idscan.py [--all \| --range A..B \| --paths F...]` | identifiers, by the place they are found |
| `plants.py SPEC.json` | planted breaks, each in its own copy of the tree |
| `release_body.py VERSION REV` | a release's body, from its release-notes report |
| `publication.py VERSION COMMIT BODY` | ten checks of a release's publication |
| `consumer.py VERSION` | a module at the floor that uses the release: `agentproc`'s taps, `ClientRefusals`, `AgentRefusals`, `Kind`, `IsMethodNotFound`, `PickPermissionOption`, `CompactionTimeline`, `ExtensionMux`, `TraceMeta`, and an elicitation form's `Fields`, `ParseValue` and `Check`, `Process.WaitContext`, `EndedCleanly`, `ToolCallTimeline`, `clienthost`'s `Files` and `Terminals`, an `InProcess` agent through `ServeAgent` with a `Transcript` as its taps, and a child's stderr tail, and in its `go test`, `acptest.Connect` with `WithElicitationCheck` and `PresentNotice`, and a transcript read with `ReadRecords`, checked with `ValidateGolden` and `Decode`, and replayed with `ConnectReplay`, so `v1.26.0` or later |
| `interop.py [--ts REV] [--py REV] [--tree DIR] [PAIR...]` | package `remote` against the official TypeScript and Python SDKs, each cloned at a pinned revision into the cache: `example/remote dial` against each one's example server, and their example clients against `example/remote serve`, over Streamable HTTP and WebSocket, eight pairs; needs `node`, `npm`, `npx` and `uv`, and `localhost:8000` free; macOS or Linux |
| `watch_ci.py REV [--branch B]` | every CI run of a commit, or of one branch, waited for, watched, and read once every job has settled (`--settle`, 180 s, `--every`, 10 s) |
| `ci_jobs.py tally \| failure RUN \| env RUN VAR` | CI's job history: every job's first attempt counted by job and conclusion; each failed job's step, seed and crash; a variable's value in each step's environment, checked with `--expect` and `--others`. Each log is read by its job id, so it is that job's own attempt |
| `find.py lines \| files` | lines or files, without `grep` |
| `procs.py TEXT...` | processes, without `pgrep` |

`gates.py --help` lists the gates. A plant in `plants.py`'s list is:

```json
{"name": "Kill running Close",
 "edits": [{"file": "agentproc/agentproc.go", "old": "...", "new": "..."}],
 "pkg": "./agentproc", "run": "^TestKill$", "want": "Kill took longer than",
 "host": "windows"}
```

`host` is optional. Each `old` must occur once in its file, and the plant
must fail its test with `want` in the output.

## Testing the tools

`make check-tools` compiles every tool and runs `tools/test_tools.py`, which
needs neither a host nor the network; CI runs it in its records step. What
needs a host or the network is run by the step that changes it, and recorded
in its PLAN.
