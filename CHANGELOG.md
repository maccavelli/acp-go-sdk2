# Changelog

## Unreleased

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
