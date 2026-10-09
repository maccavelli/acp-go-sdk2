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
| 0005 | PLAN | [Implement the separation from github.com/coder/acp-go-sdk](decisions/0005-PLAN-separate-module-from-coder.md) | proposed |
| 0006 | MADR | [Emit generated Go with go/ast instead of Jennifer](decisions/0006-MADR-emit-go-with-go-ast.md) | proposed |
| 0006 | PLAN | [Implement go/ast emission](decisions/0006-PLAN-emit-go-with-go-ast.md) | proposed |

## I want to…

| I want to… | Start here |
| :--- | :--- |
| see whether a Go 1.27.1 floor fits this SDK | [0001-REPORT-go-127-acp-parity.md](reports/0001-REPORT-go-127-acp-parity.md) |
| see how the generated API lines up with the current ACP spec | [0001-REPORT-go-127-acp-parity.md](reports/0001-REPORT-go-127-acp-parity.md) |
| know why Go was accepted at 1.27.2 with schema 1.24.1 | [0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md](decisions/0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md) |
| know why the schema pin is 1.25.0 | [0004-MADR-pin-acp-schema-1-25-0.md](decisions/0004-MADR-pin-acp-schema-1-25-0.md) |
| know why this tree is not the Coder Go SDK | [0005-MADR-separate-module-from-coder.md](decisions/0005-MADR-separate-module-from-coder.md) |
| rename the module off github.com/coder/acp-go-sdk | [0005-PLAN-separate-module-from-coder.md](decisions/0005-PLAN-separate-module-from-coder.md) |
| know why the generator would stop using Jennifer | [0006-MADR-emit-go-with-go-ast.md](decisions/0006-MADR-emit-go-with-go-ast.md) |
| emit the schema bindings with go/ast | [0006-PLAN-emit-go-with-go-ast.md](decisions/0006-PLAN-emit-go-with-go-ast.md) |
| apply the Go 1.27.2 and schema 1.25.0 pin | [0004-PLAN-pin-acp-schema-1-25-0.md](decisions/0004-PLAN-pin-acp-schema-1-25-0.md) |
| understand how this fits together | [architecture.md](architecture.md) |
| know the rules agents follow in this repository | [AGENTS.md](../AGENTS.md) |
| know why this repository has the fleet workspace and library CI | [0003-MADR-install-fleet-agent-workspace.md](decisions/0003-MADR-install-fleet-agent-workspace.md) |
| install the fleet workspace and library CI | [0003-PLAN-install-fleet-agent-workspace.md](decisions/0003-PLAN-install-fleet-agent-workspace.md) |
