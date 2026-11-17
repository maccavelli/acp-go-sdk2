# Changelog

## Unreleased

- Made `agentproc`'s tests that need an agent's exit seen within a grace run under a grace of 3 s (`exitGrace`), and
  rescaled five bounds that are not a grace (`burstWait` 35 s, `tapErrorWait` 39 s, `tappedFramesGrace` 28 s,
  `lastOutputGrace` 42 s, `releasedWait` 35 s), so a loaded runner under `-race` no longer fails them; tests only.
- Made the four `agentproc` tests that read 24 to 56 MiB through the connection (`TestBlockedTapDetached`,
  `TestDetachedTapIsPrefix`, `TestInProcessDetachedTapIsPrefix`, `TestTapDrainBoundedByGrace`) read them one test at a
  time, under a test-only lock, so that the Windows laptop's package run and a four-CPU Linux host beside the module no
  longer wait past their 30 s bound for each other's reads under `-race`, and the two 56 MiB tests hold it until their
  `Close` has returned, so that no other of the four reads beside `Close`'s first step, which ended at its 5 s grace without
  the agent's exit seen in 4 of 100 module rounds with the lock released earlier; tests only. `agentproc`'s time in CI's step
  without `-race` rose from about 8 s to about 20 s with the graces of the item above: seventeen tests and subtests each
  sit a grace out by design, and the grace went from 300 ms to 3 s
  ([0057-MADR](docs/decisions/0057-MADR-agentproc-test-graces-on-a-loaded-runner.md)).
- Added the documentation defect list,
  [0056-REPORT-documentation-defect-list.md](docs/reports/0056-REPORT-documentation-defect-list.md): every statement
  in the repository's docs that the code contradicts, every feature no doc describes, and every exported member with no
  doc comment, each with the phase of the docs plan that fixes it. It changes no doc and no code.
- Corrected four statements: `docs/architecture.md` named `output.go` for the terminal output's file (it is
  `terminals.go`) and gave the wrong test counts for `stderr_test.go` (seven) and package `remote` (64);
  `README.md` said every example client advertises the file-system capability, but `example/remote`'s `dial` advertises
  no capability.
- Added this file. `RELEASING.md` now names it as the changelog, beside the release-notes reports, and describes the
  release tools as they are used.
- Documented what no doc described: `README.md` now covers the connection's logging (`WithLogger`, `SetLogger`), the
  drop handler, the `ConnectionOptions` of `agentproc`, the five remaining error constructors, `ToolCallStream.Entity`
  and `ErrToolCallStreamFinished`, and the example programs' flags; `docs/architecture.md` the Make targets and
  variables, the generator's, the mutation tool's and the scripts' flags, and `opencode.json`; `tools/README.md` the
  tools' flags, with their defaults.
- Rewrote `docs/README.md`'s "I want to…" table as one row per task a reader comes with, each pointing to the doc or
  record that answers it now; the Records table stays the index of every record. `tools/README.md` now describes
  `consumer_probe.py --tree` and `remote.py --host`.
- Made this file docs-safe: a change to `CHANGELOG.md` alone now runs the Docs workflow and not CI's Go suite, and
  `make check-records` checks its links with the other unnumbered docs.
- Documented the exported fields and interface methods of the hand-written public code that had no doc comment:
  `CompactionEntity`, `ToolCallEntity`, `RequestError`, `ProtocolVersionError`, `ElicitationField`, `ElicitationURL`,
  `Observation`, `acptest`'s `TerminalHost`, `Pair`, `Record` and `Turn`, and `agentproc`'s `Command` and `InProcess`.
  The package comment's list of the connection's error sentinels now names `ErrShutdown`, `ErrHandlersRunning`,
  `ErrSendingMiddleware` and `ErrNotificationQueueOverflow`. Comments only: no code and no API changes. The generated
  types are the next phase's.
- Documented the generated discriminator fields: each union variant type's comment now names the field that holds its
  discriminator and the value the schema fixes it at, such as `Its Type field holds the "type" discriminator, which the
  schema fixes at "terminal".` (69 types, 2 of them in `unstable`), and `IdleStateUpdateNone`'s says the schema leaves
  `StopReason` absent or null. `Negotiation`'s comment names `Request` and `Response`. The generator writes them:
  comments only, no API change.

## v1.36.0 — 2026-11-11

57 compatible additions and nothing incompatible against v1.35.0, so a minor release. The ACP schema pin moves from
`1.25.0` to `1.31.0`. It adds the client system prompt, stable since schema `1.30.0`, with its capability gates and
its check, and one change on the wire, limited to session requests that carry `systemPrompt`. The `go` line moves to
`1.26.11`. Details are in
[the release-notes report](docs/reports/0054-REPORT-release-notes-v1-36-0.md).

---

Releases before v1.36.0 are described by the release-notes reports listed in [docs/README.md](docs/README.md)
(`docs/reports/*-REPORT-release-notes-v*.md`). v1.14.0's notes are filed as `0019-REPORT-release-notes-v0-18-0.md`,
v1.13.0 was retracted (`go.mod`), and tags before v0.15.0 have no notes.
