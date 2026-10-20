# Documentation

This tree holds decisions and observations about the SDK. Guides are not in the tree yet. The first guide creates `docs/guides/`.

- **[architecture.md](architecture.md)** — how the pieces fit together, as they are now.
- **[decisions/](decisions/)** — numbered MADR and PLAN pairs.
- **[reports/](reports/)** — what was observed, deciding nothing.

## Records

| Number | Kind | Record | Status |
| :--- | :--- | :--- | :--- |
| 0001 | REPORT | [Go 1.27.1 floor and ACP spec parity](reports/0001-REPORT-go-127-acp-parity.md) | observed |
| 0002 | MADR | [Pin Go 1.27.2 and ACP schema 1.24.1](decisions/0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md) | accepted |
| 0002 | PLAN | [Implement the Go 1.27.2 and schema 1.24.1 pin](decisions/0002-PLAN-pin-go-1-27-2-and-acp-schema-1-24-1.md) | superseded |
| 0003 | MADR | [Install the fleet workspace and library CI](decisions/0003-MADR-install-fleet-agent-workspace.md) | accepted |
| 0003 | PLAN | [Implement the fleet workspace and library CI](decisions/0003-PLAN-install-fleet-agent-workspace.md) | complete |
| 0004 | MADR | [Pin ACP schema 1.25.0 on the Go 1.27.2 floor](decisions/0004-MADR-pin-acp-schema-1-25-0.md) | accepted |
| 0004 | PLAN | [Implement the schema 1.25.0 pin](decisions/0004-PLAN-pin-acp-schema-1-25-0.md) | complete |
| 0005 | MADR | [Separate this module from github.com/coder/acp-go-sdk](decisions/0005-MADR-separate-module-from-coder.md) | accepted |
| 0005 | PLAN | [Implement the separation from github.com/coder/acp-go-sdk](decisions/0005-PLAN-separate-module-from-coder.md) | complete |
| 0006 | MADR | [Emit generated Go with go/ast instead of Jennifer](decisions/0006-MADR-emit-go-with-go-ast.md) | accepted |
| 0006 | PLAN | [Implement go/ast emission](decisions/0006-PLAN-emit-go-with-go-ast.md) | complete |
| 0007 | MADR | [Keep this SDK self-contained on the current ACP schema](decisions/0007-MADR-self-contained-acp-sdk.md) | accepted |
| 0007 | PLAN | [Implement the self-contained SDK](decisions/0007-PLAN-self-contained-acp-sdk.md) | complete |
| 0008 | REPORT | [Review of the self-contained SDK records, a go/ast generator, and encoding/json/v2](reports/0008-REPORT-self-contained-sdk-review.md) | observed |
| 0009 | MADR | [Fix the generated output on encoding/json v1](decisions/0009-MADR-fix-generated-output-on-json-v1.md) | accepted |
| 0009 | PLAN | [Implement the generated-output fixes](decisions/0009-PLAN-fix-generated-output-on-json-v1.md) | complete |
| 0010 | MADR | [Bring the user-facing docs in line with the current tree](decisions/0010-MADR-align-docs-with-current-tree.md) | accepted |
| 0010 | PLAN | [Implement the docs update](decisions/0010-PLAN-align-docs-with-current-tree.md) | complete |
| 0011 | MADR | [Harden the code generator into a validated, schema-driven engine](decisions/0011-MADR-harden-code-generator-engine.md) | accepted |
| 0011 | PLAN | [Implement phase 1: the resolved schema model](decisions/0011-PLAN-resolved-schema-model.md) | complete |
| 0011 | PLAN | [Implement phase 2: the wire fixes](decisions/0011-PLAN-wire-fixes.md) | complete |
| 0011 | PLAN | [Implement phase 3: the protocol table](decisions/0011-PLAN-protocol-table.md) | complete |
| 0011 | PLAN | [Implement phase 5: safe output](decisions/0011-PLAN-safe-output.md) | complete |
| 0011 | PLAN | [Implement phase 6: tests of the engine](decisions/0011-PLAN-engine-tests.md) | complete |
| 0012 | MADR | [Generate the ACP schema faithfully through a keyword registry](decisions/0012-MADR-schema-fidelity-framework.md) | accepted |
| 0012 | PLAN | [Implement the schema fidelity framework](decisions/0012-PLAN-schema-fidelity-framework.md) | complete |
| 0013 | MADR | [Close the follow-ups phase 6 left](decisions/0013-MADR-close-phase-6-follow-ups.md) | accepted |
| 0013 | PLAN | [Implement the phase 6 follow-ups](decisions/0013-PLAN-close-phase-6-follow-ups.md) | complete |
| 0013 | REPORT | [Release notes for v0.15.0, against v0.14.0](reports/0013-REPORT-release-notes-v0-15-0.md) | observed |
| 0014 | MADR | [Union constructors take every required key, cover the catch-all and every union](decisions/0014-MADR-union-constructors-take-every-required-key.md) | accepted |
| 0014 | PLAN | [Implement the union constructor decisions](decisions/0014-PLAN-union-constructors-take-every-required-key.md) | complete |
| 0014 | REPORT | [Release notes for v0.16.0, against v0.15.0](reports/0014-REPORT-release-notes-v0-16-0.md) | observed |
| 0015 | MADR | [Enforce absolute protocol paths, with a consumer validation hook](decisions/0015-MADR-validate-absolute-protocol-paths.md) | accepted |
| 0015 | PLAN | [Implement absolute protocol paths, with consumer hooks](decisions/0015-PLAN-validate-absolute-protocol-paths.md) | complete |
| 0016 | REPORT | [ACP v1 spec adherence audit](reports/0016-REPORT-acp-v1-conformance-audit.md) | observed |
| 0017 | MADR | [Remediate the ACP v1 conformance violations: cancellation, JSON-RPC errors, version and capabilities](decisions/0017-MADR-remediate-acp-v1-conformance-violations.md) | accepted |
| 0017 | PLAN | [Implement the ACP v1 conformance remediation](decisions/0017-PLAN-remediate-acp-v1-conformance-violations.md) | complete |
| 0017 | REPORT | [Release notes for v0.17.0, against v0.16.0](reports/0017-REPORT-release-notes-v0-17-0.md) | observed |
| 0018 | MADR | [A side connection assigns its Connection before the Connection starts reading](decisions/0018-MADR-start-side-connections-after-assignment.md) | accepted |
| 0018 | PLAN | [Implement the side connections' start after assignment](decisions/0018-PLAN-start-side-connections-after-assignment.md) | complete |
| 0018 | REPORT | [Release notes for v0.17.1, against v0.17.0](reports/0018-REPORT-release-notes-v0-17-1.md) | observed |
| 0019 | MADR | [Remediate the defects, gaps and idiom drift the 2026-10-13 debugging pass found](decisions/0019-MADR-remediate-debugging-pass-findings.md) | accepted |
| 0019 | PLAN | [Implement phase 1 of 0019: the behaviour fixes, with no exported change](decisions/0019-PLAN-behaviour-fixes.md) | complete |
| 0019 | PLAN | [Implement phase 2 of 0019: the API and behaviour decisions, released with phase 1 as v0.18.0](decisions/0019-PLAN-api-decisions.md) | complete |
| 0019 | PLAN | [Implement phase 3 of 0019: generator hardening](decisions/0019-PLAN-generator-hardening.md) | complete |
| 0019 | PLAN | [Implement phase 4 of 0019: examples, documentation, tests and tooling](decisions/0019-PLAN-examples-docs-tests-tooling.md) | complete |
| 0019 | PLAN | [Implement the fixes for issues #7 to #9 under 0019, before phase 5](decisions/0019-PLAN-issue-fixes.md) | complete |
| 0019 | PLAN | [Implement phase 5 of 0019: idioms and performance](decisions/0019-PLAN-idioms-performance.md) | complete |
| 0019 | REPORT | [Release notes for v1.14.0, against v0.17.1](reports/0019-REPORT-release-notes-v0-18-0.md) | observed |
| 0019 | REPORT | [Release notes for v1.15.0, against v1.14.0](reports/0019-REPORT-release-notes-v1-15-0.md) | observed |
| 0019 | REPORT | [Release notes for v1.15.1, against v1.15.0](reports/0019-REPORT-release-notes-v1-15-1.md) | observed |
| 0019 | REPORT | [Release notes for v1.15.2, against v1.15.1](reports/0019-REPORT-release-notes-v1-15-2.md) | observed |
| 0019 | REPORT | [Release notes for v1.16.0, against v1.15.2](reports/0019-REPORT-release-notes-v1-16-0.md) | observed |
| 0020 | MADR | [Release phases 1 and 2 of 0019 as v1.14.0, retracting the accidental v1.13.0](decisions/0020-MADR-retract-accidental-v1-tag.md) | accepted |
| 0020 | PLAN | [Implement the v1.14.0 release and the retraction of v1.13.0](decisions/0020-PLAN-retract-accidental-v1-tag.md) | complete |
| 0021 | MADR | [Resolve issues #4 and #5 before v1.14.0: a prompt-overlap policy option and a three-state Patch[T] for patch fields](decisions/0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md) | accepted |
| 0021 | PLAN | [Implement the prompt-overlap policy option and the three-state Patch[T] before v1.14.0](decisions/0021-PLAN-resolve-issues-4-and-5-before-v1-14-0.md) | complete |
| 0022 | MADR | [Keep the order of an elicitation form's properties before v1.15.0: a generated PropertyOrder beside the map](decisions/0022-MADR-keep-elicitation-property-order-before-v1-15-0.md) | accepted |
| 0022 | PLAN | [Implement a generated PropertyOrder that keeps an elicitation form's property order, before v1.15.0](decisions/0022-PLAN-keep-elicitation-property-order-before-v1-15-0.md) | complete |
| 0023 | MADR | [Leave a connection's lifetime to the application, make Close end every handler, and fail the tests on a leaked connection](decisions/0023-MADR-connection-lifetime-and-test-leaks.md) | accepted |
| 0023 | PLAN | [Implement 0023: Close ends every handler, the leaking tests close their connections, and a leak guard](decisions/0023-PLAN-connection-lifetime-and-test-leaks.md) | complete |
| 0023 | REPORT | [Release notes for v1.16.1, against v1.16.0](reports/0023-REPORT-release-notes-v1-16-1.md) | observed |
| 0024 | MADR | [Bring the SDK in line with Go SDK best practices, on a Go 1.26.9 floor](decisions/0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md) | accepted |
| 0024 | PLAN | [Implement phase 1 of 0024: the Go 1.26.9 floor](decisions/0024-PLAN-go-1-26-9-floor.md) | complete |
| 0024 | REPORT | [Release notes for v1.16.2, against v1.16.1](reports/0024-REPORT-release-notes-v1-16-2.md) | observed |
| 0024 | PLAN | [Implement phase 2 of 0024: documentation](decisions/0024-PLAN-documentation.md) | complete |
| 0024 | REPORT | [Release notes for v1.16.3, against v1.16.2](reports/0024-REPORT-release-notes-v1-16-3.md) | observed |
| 0024 | PLAN | [Implement phase 3 of 0024: behaviour fixes](decisions/0024-PLAN-behaviour-fixes.md) | complete |
| 0024 | REPORT | [Release notes for v1.17.0, against v1.16.3](reports/0024-REPORT-release-notes-v1-17-0.md) | observed |
| 0024 | PLAN | [Implement phase 4 of 0024: the API gate and deprecation notices](decisions/0024-PLAN-api-gate-and-deprecation.md) | in-progress |
| 0024 | REPORT | [Release notes for v1.17.1, against v1.17.0](reports/0024-REPORT-release-notes-v1-17-1.md) | observed |

## I want to…

| I want to… | Start here |
| :--- | :--- |
| see whether a Go 1.27.1 floor fits this SDK | [0001-REPORT-go-127-acp-parity.md](reports/0001-REPORT-go-127-acp-parity.md) |
| see how the generated API lines up with the current ACP spec | [0001-REPORT-go-127-acp-parity.md](reports/0001-REPORT-go-127-acp-parity.md) |
| see what the current ACP v1 spec audit found | [0016-REPORT-acp-v1-conformance-audit.md](reports/0016-REPORT-acp-v1-conformance-audit.md) |
| know why Go was accepted at 1.27.2 with schema 1.24.1 | [0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md](decisions/0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md) |
| know why the schema pin is 1.25.0 | [0004-MADR-pin-acp-schema-1-25-0.md](decisions/0004-MADR-pin-acp-schema-1-25-0.md) |
| know why this tree is not the Coder Go SDK | [0005-MADR-separate-module-from-coder.md](decisions/0005-MADR-separate-module-from-coder.md) |
| rename the module off github.com/coder/acp-go-sdk | [0005-PLAN-separate-module-from-coder.md](decisions/0005-PLAN-separate-module-from-coder.md) |
| know why the generator uses go/ast instead of Jennifer | [0006-MADR-emit-go-with-go-ast.md](decisions/0006-MADR-emit-go-with-go-ast.md) |
| emit the schema bindings with go/ast | [0006-PLAN-emit-go-with-go-ast.md](decisions/0006-PLAN-emit-go-with-go-ast.md) |
| keep this SDK self-contained, off the previous module and off Jennifer | [0007-MADR-self-contained-acp-sdk.md](decisions/0007-MADR-self-contained-acp-sdk.md) |
| apply the separation from the previous project | [0007-PLAN-self-contained-acp-sdk.md](decisions/0007-PLAN-self-contained-acp-sdk.md) |
| see whether 0005, 0006, and 0007 hold against the tree | [0008-REPORT-self-contained-sdk-review.md](reports/0008-REPORT-self-contained-sdk-review.md) |
| read a go/ast generator that reproduces the Jennifer output | [0008-REPORT-self-contained-sdk-review.md](reports/0008-REPORT-self-contained-sdk-review.md) |
| know why the SDK stays on encoding/json v1 | [0007-MADR-self-contained-acp-sdk.md](decisions/0007-MADR-self-contained-acp-sdk.md) |
| see the v1 and v2 measurements behind that | [0008-REPORT-self-contained-sdk-review.md](reports/0008-REPORT-self-contained-sdk-review.md) |
| know why the generated output changed after the go/ast port | [0009-MADR-fix-generated-output-on-json-v1.md](decisions/0009-MADR-fix-generated-output-on-json-v1.md) |
| see how unset unions, nil required arrays, and union docs were fixed | [0009-PLAN-fix-generated-output-on-json-v1.md](decisions/0009-PLAN-fix-generated-output-on-json-v1.md) |
| apply the Go 1.27.2 and schema 1.25.0 pin | [0004-PLAN-pin-acp-schema-1-25-0.md](decisions/0004-PLAN-pin-acp-schema-1-25-0.md) |
| understand how this fits together | [architecture.md](architecture.md) |
| know what the SDK sends on the wire | [architecture.md](architecture.md) |
| know why the user-facing docs were brought up to date | [0010-MADR-align-docs-with-current-tree.md](decisions/0010-MADR-align-docs-with-current-tree.md) |
| see what is wrong with the code generator and how it would be hardened | [0011-MADR-harden-code-generator-engine.md](decisions/0011-MADR-harden-code-generator-engine.md) |
| build the generator's resolved schema model | [0011-PLAN-resolved-schema-model.md](decisions/0011-PLAN-resolved-schema-model.md) |
| fix how the generated unions read and write unknown, absent, and null values | [0011-PLAN-wire-fixes.md](decisions/0011-PLAN-wire-fixes.md) |
| see where the generator keeps the protocol behaviour the schema does not express | [0011-PLAN-protocol-table.md](decisions/0011-PLAN-protocol-table.md) |
| see how the generator checks its output before writing it | [0011-PLAN-safe-output.md](decisions/0011-PLAN-safe-output.md) |
| see how the generator and the generated SDK are tested | [0011-PLAN-engine-tests.md](decisions/0011-PLAN-engine-tests.md) |
| see how each schema keyword reaches the generated Go code | [0012-MADR-schema-fidelity-framework.md](decisions/0012-MADR-schema-fidelity-framework.md) |
| know the rules agents follow in this repository | [AGENTS.md](../AGENTS.md) |
| know why this repository has the fleet workspace and library CI | [0003-MADR-install-fleet-agent-workspace.md](decisions/0003-MADR-install-fleet-agent-workspace.md) |
| install the fleet workspace and library CI | [0003-PLAN-install-fleet-agent-workspace.md](decisions/0003-PLAN-install-fleet-agent-workspace.md) |
