# Architecture

How this repository is put together, as it is now. This file carries no
history and no rationale — when you want to know *why* something is this
way, the decision records under [decisions/](decisions/) hold the argument,
and [README.md](README.md) points at the ones people ask for most.

## What it is

This repository is the Go module `github.com/maccavelli/acp-go-sdk2`. Hand-written
session and transport code sits at the repository root. `cmd/generate` reads
`schema/` and writes the `*_gen.go` files. `example/` holds agent and client
programs. `testdata/` holds fixtures. GitHub Actions runs
`.github/workflows/ci.yml`. The schema stamp lives in `schema/version`. The
module `go` line lives in `go.mod` and `cmd/generate/go.mod`.

## Tree

```text
README.md                 repository entry; links here
AGENTS.md                 agent instructions
.markdownlint-cli2.jsonc  fleet Markdown style
.gitattributes            LF on every platform
scripts/check_records.py  record numbering and link check
docs/
  README.md               ToC and the "I want to…" matrix
  architecture.md         this file
  decisions/              MADR/PLAN pairs
  reports/                numbered observations
  guides/                 unnumbered how-to documents (not created yet)
```

## What is not here

`docs/guides/` does not exist. The first guide creates it. The Go floor and
the schema pin live in the decision records indexed from `docs/README.md`.
