---
status: accepted
date: 2026-10-10
decision-makers: Repository owner
---

# Bring the user-facing docs in line with the current tree

## Context and Problem Statement

[0005-PLAN-separate-module-from-coder.md](0005-PLAN-separate-module-from-coder.md), [0006-PLAN-emit-go-with-go-ast.md](0006-PLAN-emit-go-with-go-ast.md),
[0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md), and [0009-PLAN-fix-generated-output-on-json-v1.md](0009-PLAN-fix-generated-output-on-json-v1.md)
are complete. They renamed the module, replaced the generator's emitter, kept the SDK on `encoding/json` v1, and changed what the generated code sends.
The decision records describe each change. The documents a user or maintainer reads first do not, and some of them are wrong about the tree at `f8244c9`:

* `README.md` tells client authors to implement `acp.ClientTerminal` for terminal features. No such type exists at schema `1.25.0`. The terminal methods
  (`CreateTerminal`, `KillTerminal`, `ReleaseTerminal`, and the rest) are part of `acp.Client`.
* `README.md` lists the flags of `example/gemini` as `-model`, `-sandbox`, `-debug`, and `-gemini`. The program also defines `-yolo`.
* `README.md` says nothing about how the SDK encodes messages. Since 0009 an unset union fails to encode with an error naming it, a nil required array or
  map is sent as `[]` or `{}`, and a non-string discriminator fails to decode with an error naming the union and the key.
* `RELEASING.md` uses `0.4.3` as the example schema version and `v0.4.3` as the example module tag. The schema stamp is `1.25.0`, downloaded as
  `schema-v1.25.0`, and `v0.4.3` is an existing tag. Its publish step does not ask for wire-level changes in the release notes, which 0009's rollout requires.
* `docs/architecture.md` lists only the workspace files. It does not name the root package's hand-written and generated files, the generator's own module
  and packages, the `encoding/json` v1 rule, the tests that guard the generated code, or the gates.
* `docs/README.md` still words three rows of its "I want to…" table as future work: "know why the generator would stop using Jennifer", "know why the
  generated output would change after the go/ast port", and "fix unset unions, nil required arrays, and union docs in the generated code".

The package doc comments (`doc.go`, `cmd/generate/main.go`, `cmd/generate/internal/emit/doc.go`) and `AGENTS.md` match the tree.

## Decision Drivers

* A reader of `README.md` or `docs/architecture.md` should not need the decision records to learn what the SDK does now.
* `docs/architecture.md` describes the present and carries no history; the records carry the rationale.
* Every name, path, flag, and behavior a document states must exist in the tree.
* The decision records are historical and are not rewritten.

## Considered Options

* Update `README.md`, `RELEASING.md`, `docs/architecture.md`, and `docs/README.md` to the tree as it is now
* Correct only the factual errors in `README.md` and `RELEASING.md`
* Leave the documents until the next release

## Decision Outcome

Chosen option: "Update `README.md`, `RELEASING.md`, `docs/architecture.md`, and `docs/README.md` to the tree as it is now", because it fixes the wrong
statements and also gives readers the encoding behavior and the layout that 0005 to 0009 established, without sending them into the records.

### Consequences

* Good, because a client author no longer looks for an interface that does not exist.
* Good, because the encoding behavior a peer can observe is stated where users read, not only in a decision record.
* Good, because `docs/architecture.md` names every part of the tree and the checks that guard it.
* Neutral, because only Markdown changes. No Go file, generated file, or test changes.
* Bad, because the documents now state more facts about the tree, and each of them can go stale the same way.

### Confirmation

* Every type, function, file, flag, and Make target the changed documents name exists in the tree, checked by a script.
* `make check-records` and `make markdownlint` exit 0, and an identifier scan of the changed files is clean.

## Pros and Cons of the Options

### Update `README.md`, `RELEASING.md`, `docs/architecture.md`, and `docs/README.md` to the tree as it is now

* Good, because all four documents describe the same tree.
* Bad, because it is the largest change of the three.

### Correct only the factual errors in `README.md` and `RELEASING.md`

* Good, because it is small.
* Bad, because `docs/architecture.md` keeps omitting the product, and the encoding behavior stays in the records only.

### Leave the documents until the next release

* Good, because it costs nothing now.
* Bad, because the README keeps pointing client authors at a type that does not exist.

## More Information

The owner accepted this option on 2026-10-10.

Implementation is [0010-PLAN-align-docs-with-current-tree.md](0010-PLAN-align-docs-with-current-tree.md).

Found and left alone: `communique.toml`, the release-notes tool's configuration, describes the project as `acp-go-sdk`. It is product configuration, not
documentation, and is outside this decision.
