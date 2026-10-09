# MADR and PLAN: load the skill, and gate mutating work

**Normative text lives in `AGENTS.md`**, section "MADR and PLAN before mutating
work". This file carries only the skill name and the gate. Do not restate the
workflow here.

## The skill

**Whenever the user asks for an MADR and a plan, load the
`madr-and-plan-writing` skill first** and follow it for authoring, naming
(`NNNN-MADR-*` / `NNNN-PLAN-*`) and review, for a fresh pair and for amending
an existing one.

The name is exact. A mistyped one returns `Unknown skill` and fails quietly.
The filesystem outranks this paragraph:

```bash
ls -d ~/.agents/skills/*madr* && grep '^name:' ~/.agents/skills/*madr*/SKILL.md
```

## The gate

**Mutating work needs an approved `docs/decisions/NNNN-MADR-*` /
`docs/decisions/NNNN-PLAN-*` pair first**; read-only investigation does not.
What counts as mutating, the approval order, the bootstrap exception and the
pre-add checks are in `AGENTS.md`. Read them there rather than trusting a
summary.
