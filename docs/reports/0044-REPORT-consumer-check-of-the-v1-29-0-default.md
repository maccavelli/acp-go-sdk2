# Consumer check of v1.29.0's default: neither consumer is affected, and both can stop dropping

Observed 2026-10-31, on this repository's `3d552f4`, for
[0044-PLAN-notification-queue-back-pressure.md](../decisions/0044-PLAN-notification-queue-back-pressure.md)'s phase 5. For each of
the SDK's two known consumers, this report finds every connection the SDK builds for it. For each, it says whether `v1.29.0`'s
default, `OverflowBlock`
([0044-MADR-notification-queue-back-pressure.md](../decisions/0044-MADR-notification-queue-back-pressure.md)), reaches it and what
it changes. It then measures each consumer's suite on `v1.29.0`, and with its own `OverflowDropNewest` replaced by `OverflowBlock`.

It decides nothing. Each consumer adopts under its own records; the issue opened in each is the list for them.

- **The consumers:**
  - magic-cli-remote at `7998b02f`, its tree clean;
  - gobble-cli at `2c13842` for the scan and the bump, and at `31e3c1c` for the trial of the switch, with uncommitted edits to its
    index and its 0035 pair in both.
- **Their pins:** both pin `v1.28.0`.

## Answer

**Neither consumer is affected by the change of default, and each can adopt `OverflowBlock` to stop dropping.**

- **Their production client connections name `OverflowDropNewest`,** so the new default does not reach them. Each chose it
  because the old default closed the connection.
- **The connections that do take the default** are gobble-cli's agent side, which a client sends almost no notifications, and
  test connections. Every test passes on `v1.29.0` as on `v1.28.0`.
- **Switching each client connection to `OverflowBlock` breaks exactly one test in each consumer,** the one that asserts a drop.
  Every other test in the packages tried passes.
- **The handlers read keep `OverflowBlock`'s rule.** No notification handler waits for a request the agent sends later.
- **An issue in each consumer** proposes the switch: magic-cli-remote
  [#32](https://github.com/maccavelli/magic-cli-remote/issues/32), and gobble-cli's [#8](https://github.com/maccavelli/gobble-cli/issues/8).

## The connections

| Consumer | Connection | Site | Policy | What `v1.29.0` changes |
| :--- | :--- | :--- | :--- | :--- |
| magic-cli-remote | the client of every engine, through `agentproc.Command` | `internal/provider/acpagent/acpagent.go:531`, options from `clientConnOptions` at `:1133` | `OverflowDropNewest`, named | nothing |
| magic-cli-remote | a test agent binary | `test/supervisors/probeagent/main.go:143` | the default | `OverflowBlock`; a test client sends it no flood |
| magic-cli-remote | test clients | `stalledpump_test.go:91`, `:326`; `conformance_test.go:117`; `rewind_test.go:100`; `automode_test.go:102`; `wireagent_test.go:74` | the default at `:91` when no options are passed; the rest as written | `TestAStalledPumpIsContainedNotHung`'s connection pauses where it closed; it passes |
| gobble-cli | the client of the in-process agent, through `agentproc.InProcess` | `acpclient/conn.go:125-136` | `OverflowDropNewest`, named, with `Options.OnDrop` and `Options.MaxQueued` | nothing |
| gobble-cli | the agent served to it | `acpserver/serve.go:45` | the default | `OverflowBlock`; its only notification is `session/cancel` |
| gobble-cli | test pairs and scripted agents | `acpclient/acptest/pair.go:44-45`, `script.go:75`, and the tests | the default | `OverflowBlock`; no test floods them |

## The handlers and the rule

`OverflowBlock`'s rule: a notification handler must not wait for a request or a `$/cancel_request` the peer sends later, nor for
code that waits for one.

- **magic-cli-remote.** The client's update path, `deliver`, hands an event over or parks it; it never blocks on its consumer.
  It faults the session at its overflow cap (its 0166-MADR D3). The drop handler `recordDropped` and the validation hook
  `noteInbound` only store (its 0207-MADR D12). Nothing in the path waits for the agent.
- **gobble-cli.** `handler.SessionUpdate` (`acpclient/conn.go:382`) calls the session's callback.
  - The callbacks are `printSink.on` (`internal/cli/print.go:77`) and `lineSession.onUpdate` (`internal/cli/line.go:406`), which
    calls `ambient.note` and `view.on` (`internal/cli/view.go:113`). Each takes its own lock and writes its output.
  - The permission prompt (`internal/cli/permission.go:247`) runs on the request's goroutine and does not hold the view's lock
    while it waits for the user.
  - The agent's `Cancel` (`acpserver/agent.go:961`) takes a lock and cancels a context.

## The runs

Each run was in a scratch copy, without the consumer's `.git`; no consumer file was changed.

**The bump.** `go build ./...`, `go vet ./...` and `go test -count=1 ./...`, at the pin and with `go get` of `v1.29.0`:

| Consumer | `v1.28.0` | `v1.29.0` |
| :--- | :--- | :--- |
| magic-cli-remote | build and vet pass; every test passes but `TestTrackedFilesUseOnlyPlaceholderHomes` | the same |
| gobble-cli | build, vet and every test pass | the same |

`TestTrackedFilesUseOnlyPlaceholderHomes` reads the git index, which the copy did not have: "no git index". It fails the same way
on both versions.

magic-cli-remote's seven stalled-pump and drop tests, `go test -race -count=5` on `v1.29.0`: 5 of 5 each, the longest
`TestACPConnectionSurvivesAStalledPump` at 0.71 s.

**The switch.** `OverflowDropNewest` replaced by `OverflowBlock` at each consumer's one production site, on `v1.29.0`,
`go test -race`:

| Consumer | Packages | Result |
| :--- | :--- | :--- |
| magic-cli-remote | `./internal/provider/acpagent/` | `TestDropHandlerRecordsTheDroppedMethod` hangs to the 3-minute timeout: it floods a handler blocked until its cleanup, and its writes now wait. Skipping it, 409 tests pass, `TestAStalledPumpIsContainedNotHung` and `TestACPConnectionSurvivesAStalledPump` among them. |
| gobble-cli | `./acpclient/...`, `./internal/cli/` | `TestDropReported` fails: "no drop reported with a queue of 1 and a blocked handler". `acpclient/acptest` and `internal/cli` pass. |

Both failing tests assert dropping, so each is replaced by a test of the new property rather than fixed. The issues say how.

## What each consumer would change

- **magic-cli-remote** ([#32](https://github.com/maccavelli/magic-cli-remote/issues/32)):
  - the pin;
  - a MADR amending its 0167-MADR D17 to D19 and 0207-MADR D12;
  - `clientConnOptions` naming `OverflowBlock` without the drop handler;
  - the per-turn drop notice retired or kept on purpose;
  - `TestDropHandlerRecordsTheDroppedMethod` replaced;
  - two stale comments, `stalledpump_test.go:18-33` and `session.go:1308-1310`.

  Its trade-off: one engine connection carries every session on that engine, so a pause holds all of them until the queue
  drains. Its `stalledpump_test.go:132-135` already names the condition for revisiting: "If upstream ever makes the reader block".
- **gobble-cli** ([#8](https://github.com/maccavelli/gobble-cli/issues/8)):
  - the pin, under its 0035 pair;
  - a MADR or an amendment of its 0004-MADR's 2026-10-01 amendment;
  - `acpclient` naming `OverflowBlock`;
  - `Options.OnDrop`, `Conn.Dropped()` and the CLI's drop warning removed or documented as opt-in only;
  - `TestDropReported` replaced;
  - its records saying that `acpserver`'s default policy is now `OverflowBlock`.

  The gain is concrete in print mode's `stream-json`, where a slowly read standard output can drop updates from the stream today.

## How it was read

Every scan and run was done by scratch scripts that only read the consumers' trees, or ran them in copies:

- the scan lists each Go file's uses of the overflow options, the drop counter and handler, and every connection constructor;
- the bump runs build, vet and the suite at the pin and at `v1.29.0`;
- the switch replaces the one `WithNotificationOverflowPolicy(acp.OverflowDropNewest)` in the named file, requiring it to match
  once.

Both issue bodies were scanned with `tools/idscan.py` before they were opened, with 0 matches.
