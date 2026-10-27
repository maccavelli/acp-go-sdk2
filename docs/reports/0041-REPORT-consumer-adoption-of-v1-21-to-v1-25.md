# Consumer adoption of v1.21.0 to v1.25.0: each site and the API that replaces it

Observed 2026-10-27, on this repository's `db92e25`, the last commit before this report, at
[0041-PLAN-close-the-gaps-the-consumers-wait-on.md](../decisions/0041-PLAN-close-the-gaps-the-consumers-wait-on.md)'s phase 15.
For each of the SDK's two known consumers, this report re-reads, at the consumer's current head, every site the investigation of
[0041-MADR-close-the-gaps-the-consumers-wait-on.md](../decisions/0041-MADR-close-the-gaps-the-consumers-wait-on.md) found. It
names the API of this SDK that replaces each, with the release that first carries it, and the consumer's records that are stale
against this repository.

It decides nothing. Each consumer adopts under its own records, and this report is the list for them.

- **The consumers:** magic-cli-remote at `5d1e7e2c`, and `<cli-b>` at `5fb6928`, with uncommitted edits to four records and an
  untracked 0034 pair. `<cli-b>` stands for the second consumer, as in
  [0035-REPORT-consumer-acp-code-for-the-sdk.md](0035-REPORT-consumer-acp-code-for-the-sdk.md).
- **Their pins:** both still pin `v1.20.0`, magic-cli-remote at `go.mod:11` and `<cli-b>` at `go.mod:15`.
- **`v1.25.0`** names the release 0041's PLAN makes in its phase 17, which is not yet tagged. The other releases are tagged, and
  each one's release-notes report names the API listed against it here:
  [0036-REPORT-release-notes-v1-21-0.md](0036-REPORT-release-notes-v1-21-0.md),
  [0038-REPORT-release-notes-v1-22-0.md](0038-REPORT-release-notes-v1-22-0.md),
  [0039-REPORT-release-notes-v1-23-0.md](0039-REPORT-release-notes-v1-23-0.md) and
  [0040-REPORT-release-notes-v1-24-0.md](0040-REPORT-release-notes-v1-24-0.md).

## Answer

**Every site the MADR found is still there, and each has an API that replaces it.**

- **magic-cli-remote:** ten sites. Four are met by `v1.22.0` and `v1.24.0`, already tagged. Six wait for `v1.25.0`.
  - terminal truncation at a character boundary;
  - `-32002` for a missing file;
  - the agent's exit order;
  - value checks before sending;
  - a tap's frames;
  - the plan content's kind.
- **`<cli-b>`:** ten sites. Four are met by `v1.21.0` to `v1.23.0`. Six wait for `v1.25.0`.
  - the bounded wait;
  - the clean-end check;
  - tool-call merging;
  - its test harness's parts;
  - `ValidateGolden`;
  - the default file and terminal client its 0031 MADR waits for.

Two of the MADR's statements about the consumers' records no longer hold as written:

- magic-cli-remote's 0207 MADR now records the releases after `v1.20.0`;
- `<cli-b>`'s harness is 891 lines without its tests, not 1,270.

Both are given below.

## How it was read

Each site was read again at the head named above, by a scratch script that prints a file's lines in a range. Each row below gives
the lines as they are at that head. The counts were made by a scratch script over the consumer's Go files:

- the harness's lines;
- its uses outside it;
- the calls of `acp.NewMethodNotFound`, `HandleExtensionMethod` and `exec.LookPath`.

No consumer file was changed.

## magic-cli-remote

All its sites are under `internal/provider/acpagent/` unless named otherwise.

| Site, at `5d1e7e2c` | What it does today | Replaced by | Since |
| :--- | :--- | :--- | :--- |
| `session.go:2593`–`:2600`, `autoAllow` | approves the option whose kind or name contains "allow" | `acp.PickPermissionOption`, by kind, never by name | `v1.22.0` |
| `elicitation.go:151` on, `formFields` and the checker to `:454` | reads and checks a form's fields by code of its own | `ElicitationSchema.Fields`, `Defaults`, `Check` and `CheckSchema`, and `ElicitationField.ParseValue` | `v1.24.0` |
| `session.go:2288` | reads a select option's `Ungrouped` values only, dropping grouped ones | `SessionConfigSelectOptions.All()` | `v1.22.0` |
| `runtime.go:342`–`:348`, `isAuthRequired` | compares a `*acp.RequestError`'s code with `-32000` | `acp.IsAuthRequired` | `v1.22.0` |
| `terminal.go:20`–`:330`; its comment at `:278` | its terminal host, which drops output "at a byte boundary (may split UTF-8 …)" | `clienthost.Terminals`, beside `clienthost.Refusals`, with its shell routing in `Command` | `v1.25.0` |
| `session.go:2648`–`:2673`, `ReadTextFile` | returns `os.ReadFile`'s error for a missing file, which the wire answers `-32603` | `clienthost.Files`, beside its own path audit; or `acp.NewResourceNotFound(path)` in its own method, which `v1.20.0` already has | `v1.25.0` for `Files` |
| `session.go:1304`, `exitPreference` | waits 500 ms after the connection's end for the agent's exit | `agentproc.Process.ExitedFirst`, and `Process.WaitContext` for a bounded wait | `v1.25.0` |
| `acpagent.go:633`–`:645`; `session.go:564`–`:570` | drops an `http` or `sse` MCP server, or an image block, the agent did not advertise, by code of its own | `Negotiation.AllowsValue`, which runs the same gates a send runs | `v1.25.0` |
| `internal/wirecap/wirecap.go:163`–`:207`, `Tap` and `lines` | splits `OutputTap`'s bytes into lines, handing `Frame` a slice of its buffer | `agentproc.LineTap`, each frame a copy | `v1.25.0` |
| `plans.go:72`–`:76`, `unknownPlanKind` | decodes an unknown plan content's raw JSON for its `type` | `PlanUpdateContent.Kind()` | `v1.25.0` |

**Its records, against this repository:**

- **`0207-MADR-move-to-current-library-releases-and-close-the-adoption-gaps.md`** is no longer stale in substance.
  - Its D24 (`:1037`) and D38 (`:1577`) still say `v1.20.0` "is the newest tag".
  - Its `:281`–`:286` now record that `v1.21.0`, `v1.22.0`, `v1.23.0` and `v1.24.0` have shipped, by `git ls-remote` on
    2026-10-27. They record the owner's choice, "Assess v1.21–v1.24 first", for a later amendment that re-proposes the pin.
  - `v1.25.0`, once tagged, is one more release for that amendment.
  - The MADR said it "still says `v1.20.0` is the newest tag". That is true of D24 and D38, and no longer of the record.
- **`0184-PLAN-adopt-acp-go-sdk2-and-offload-protocol-handling.md`** still says of `maccavelli/acp-go-sdk2#4`, at `:913`, "It is
  open". The issue was closed on 2026-10-15 at 02:56:56 UTC, and its MADR, at `:438`, says the adoption "resolves" it.

## `<cli-b>`

| Site, at `5fb6928` | What it does today | Replaced by | Since |
| :--- | :--- | :--- | :--- |
| `acpclient/conn.go:207`, `acpserver/serve.go:71`, `acpclient/acptest/pair.go:65`: three copies of `handlersDone(wait, limit)` | runs `Wait` on a goroutine and selects on a timer | `Connection.WaitContext(ctx)`; a side connection's through its `Connection()` | `v1.25.0` |
| `acpserver/serve.go:58`–`:66`, `ended` | takes an `Err()` wrapping `io.EOF` or `acp.ErrConnectionClosed` as a normal end, the overflow policy's close included, which wraps `ErrConnectionClosed` | `acp.EndedCleanly`, false for the overflow's close | `v1.25.0` |
| `acpclient/conn.go:556`–`:593`, the client handler's stubs; `acpserver/agent.go:1179` and `:1245` | answers each method it does not offer with `acp.NewMethodNotFound` by hand | `acp.ClientRefusals`; `acp.AgentRefusals`; beside a `clienthost` type, `clienthost.Refusals` | `v1.21.0`; `v1.22.0`; `v1.25.0` |
| `acpserver/extensions.go:61`–`:70`; `acpclient/conn.go:435` | dispatches its `_`-prefixed methods by a map of its own | `acp.ExtensionMux`, with `Handle` and `Notify`. It decodes with `encoding/json` v1, while this agent encodes its results with `encoding/json/v2` (`extensions.go:62`), so the fit there is partial | `v1.23.0` |
| `acpserver/rewrite.go:28`–`:33` | resolves a bare stdio MCP command with `exec.LookPath` | `acp.ResolveStdioCommands`, through `acp.WithInboundRewrite` | `v1.22.0` |
| `acpserver/agent.go:394`, `a.authMethods(…Auth.Terminal)` | filters its auth methods by the client's `auth.terminal` | `acp.OfferedAuthMethods` | `v1.22.0` |
| `internal/cli/view.go:146`–`:166` | keeps each tool call's title and status, and shows each update's content as it comes | `acp.ToolCallTimeline`: content replaced, never appended, and an unseen id started `pending` | `v1.25.0` |
| `acpclient/acptest/`: 891 lines without its tests, 1,280 with them, used 385 times outside it | its own harness: a pair, a recorder of direction, `Recorder.Wait(ctx, n)` (`recorder.go:101`), scripted terminals (`client.go:154`) and error turns | `acptest.ConnectRecorded`, `Recorder.Records` and `Recorder.Wait` (the same signature); `MemoryClient.Terminals` with a `*clienthost.Terminals`; `Turn.Err`, `Usage` and `WaitCancel`; `MemoryClient.SetElicitation` | `v1.25.0`; `Connect` since `v1.23.0` |
| `docs/decisions/0016-MADR-acp-go-sdk2-v0.15.0-and-stable-surface.md:207`, D6 | asks for `ValidateGolden(t, path)`: each recorded frame decoded into its method's type, a response by its request's, failing on a decode error, a `Validate` error, or a method with no type | `acptest.ValidateGolden(t, records)`, with its table of every method, by direction | `v1.25.0` |
| `docs/decisions/0031-MADR-sdk-current-releases-and-adoption.md:98`–`:99` | waits for "a default client for fs, terminal and elicitation"; "`<cli-b>` keeps these until the SDKs have them" | `clienthost.Files` and `clienthost.Terminals`; for elicitation, the form helpers, since showing a form is the client's own | `v1.25.0`; `v1.24.0` |

**How D6's golden files map.** `ValidateGolden` takes records with their direction, where D6 takes a path. `<cli-b>`'s goldens,
such as `acpclient/acptest/testdata/initialize.golden`, are a line a frame, prefixed `client→agent` or `agent→client`. Those are
`acptest.Sent` and `acptest.Received` as `ConnectRecorded` records the client side, so reading a golden into `[]acptest.Record`
is a few lines of its own. Its file layout stays its own, as the MADR's item 8 said.

**Its records, against this repository:**

- **`0034-MADR-pin-acp-go-sdk2-v1-21-0-and-adopt-session-update-helpers.md` and its PLAN**, both untracked at the head, target
  `v1.21.0`. `v1.24.0` is tagged and `v1.25.0` is next.
- **`0031-MADR-sdk-current-releases-and-adoption.md:98`–`:99`** keeps its stubs until the SDK has a default file, terminal and
  elicitation client. Of those, `v1.25.0` has the file and terminal ones.
- **The MADR's "1,270-line `acptest`"** is 1,280 lines with its tests and 891 without. Its "about 380 times" is 385.

## What this does not cover

- **Adoption itself:** each consumer's pin, code and records, under its own process.
- **Sites in the consumers the MADR's investigation did not find.** 0035's report lists what else is protocol-generic in them.
- **Anything that needs schema `1.26.0`,** which stays deferred.
