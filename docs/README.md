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
| 0002 | PLAN | [Implement the Go 1.27.2 and schema 1.24.1 pin](decisions/0002-PLAN-pin-go-1-27-2-and-acp-schema-1-24-1.md) | proposed |
| 0003 | MADR | [Install the fleet workspace and library CI](decisions/0003-MADR-install-fleet-agent-workspace.md) | accepted |
| 0003 | PLAN | [Implement the fleet workspace and library CI](decisions/0003-PLAN-install-fleet-agent-workspace.md) | proposed |

## I want to…

| I want to… | Start here |
| :--- | :--- |
| see whether a Go 1.27.1 floor fits this SDK | [0001-REPORT-go-127-acp-parity.md](reports/0001-REPORT-go-127-acp-parity.md) |
| see how the generated API lines up with the current ACP spec | [0001-REPORT-go-127-acp-parity.md](reports/0001-REPORT-go-127-acp-parity.md) |
| know why Go is pinned to 1.27.2 and the schema to 1.24.1 | [0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md](decisions/0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md) |
| apply that pin | [0002-PLAN-pin-go-1-27-2-and-acp-schema-1-24-1.md](decisions/0002-PLAN-pin-go-1-27-2-and-acp-schema-1-24-1.md) |
| understand how this fits together | [architecture.md](architecture.md) |
| know the rules agents follow in this repository | [AGENTS.md](../AGENTS.md) |
| know why this repository has the fleet workspace and library CI | [0003-MADR-install-fleet-agent-workspace.md](decisions/0003-MADR-install-fleet-agent-workspace.md) |
| install the fleet workspace and library CI | [0003-PLAN-install-fleet-agent-workspace.md](decisions/0003-PLAN-install-fleet-agent-workspace.md) |
