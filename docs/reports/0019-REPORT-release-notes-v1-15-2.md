# Release notes for v1.15.2, against v1.15.1

Observed 2026-10-18 on `f260ac8`, for [0019-PLAN-issue-fixes.md](../decisions/0019-PLAN-issue-fixes.md), step 6. This report drafts
the text of the GitHub release for `v1.15.2`, as `RELEASING.md` describes it. It decides nothing.

Each statement names its source:

- a record;
- a commit;
- or the exported API listing of the 0013 report's listing program, run on `git archive v1.15.1` (2429 lines) and on the tree at
  `f260ac8` (2429 lines).

Bring it up to date if more changes land before the tag. No exported API changes, so the release is a patch
([0019-PLAN-issue-fixes.md](../decisions/0019-PLAN-issue-fixes.md), "Rollout and Rollback"), which the owner confirms at the tag. Two
behaviours change, and the release text names both.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.15.2

The ACP schema is still `1.25.0` (`git diff v1.15.1 -- schema/` is empty), and the exported API is unchanged. This release fixes the
three open issues, [#7](https://github.com/maccavelli/acp-go-sdk2/issues/7), [#8](https://github.com/maccavelli/acp-go-sdk2/issues/8)
and [#9](https://github.com/maccavelli/acp-go-sdk2/issues/9), and one example bug, as
[0019-MADR-remediate-debugging-pass-findings.md](../decisions/0019-MADR-remediate-debugging-pass-findings.md) decided on 2026-10-18.

#### Behaviour changes

- **The requests received before the end of input are answered**
  ([#8](https://github.com/maccavelli/acp-go-sdk2/issues/8)). When a connection's input ends cleanly, at end of file, it no longer ends
  at once. A stdio agent whose standard input a script closes after writing its requests now answers them, where it answered none.
  (`5763a36`; B23.)
  - The requests this side is waiting on fail at once with `-32603`, wrapping `acp.ErrPeerDisconnected`, as does any it sends after the
    end. No answer can arrive.
  - The handlers already running keep their contexts, and their answers are written, for at most five seconds. Then their contexts are
    cancelled and `Done()` closes.
  - A read that fails with an error still ends the connection at once, and `Close` during the drain ends it at once.
  - An application that relied on its request handlers being cancelled as soon as the input ended sees them run to their answers, up to
    five seconds. `Done`'s doc and the README's "Closing and limits" state the new order.
- **The subagent updates are gated on `subagents`** ([#9](https://github.com/maccavelli/acp-go-sdk2/issues/9)). The unstable schema
  says: "The client must advertise this capability before the agent sends subagent updates or session-directed messages". So, without
  `subagents` in the client's capabilities, the unstable `subagent_update`, `session_message` and `session_message_chunk` updates are
  handled like the other gated updates. (`ef08400`; B24.)
  - **Sending:** refused with a `*acp.CapabilityError` naming `subagents` and the variant.
  - **Receiving:** dropped and logged, which `WithInboundValidation` can override.

#### Fixes

- **`Prompt` cancels only a prompt it may have sent** ([#7](https://github.com/maccavelli/acp-go-sdk2/issues/7)). `Prompt` called with
  a context that had already ended, or whose context ended before its request was handed to the writer, sent `session/cancel` for a
  turn the agent never received. Now it sends nothing then. A prompt that was written and then abandoned is still cancelled, now under
  `context.WithoutCancel(ctx)`, which keeps the caller's values. (`2cdc39a`; B22, I10.)
- **The example clients print a tool call's status** as its value, `updated: completed`, where they printed its pointer. (`f260ac8`;
  E18.)

#### For contributors

- A generator test requires each capability rule's quoted source to appear in the committed schema, unless it quotes a page of the
  specification, which the test then names. (`ef08400`.)

## Sources not in the release text

- **Commits since `v1.15.1` that change Go files outside `cmd/generate`**
  (`git log v1.15.1..HEAD -- '*.go' ':!cmd/generate'`): `ef08400`, `2cdc39a`, `5763a36` and `f260ac8`.
  - The SDK's own non-test files that change are `connection.go`, `client_gen.go` (`Prompt`'s call and its cancel) and `types_gen.go`
    (the three gate checks): 139 lines added and 22 removed.
  - The generator changes in `ef08400` and `2cdc39a`. The other commits change records.
- **The API listing:** 0 lines removed, 0 added, `v1.15.1` against `f260ac8`.
- **Execution records:** each change, its tests, the planted breaks each test was seen to fail on, issue #7 to #9's probes re-run on
  the fix and on the commit before it, and a Windows run for each step are in
  [0019-PLAN-issue-fixes.md](../decisions/0019-PLAN-issue-fixes.md), with its deviation of 2026-10-18 (the source guard).
- **The reference SDKs at end of input:** what they do is in the MADR's "Evaluated and decided with the owner, 2026-10-18". B23 follows
  the Rust SDK, with a bound.
- **Left to the owner:** closing issues #7 to #9 once `v1.15.2` is published.
- **Not in this release:** phase 5 of 0019 (idioms and performance), and schema `1.26.0`, which the owner decided not to pin yet.
