---
status: accepted
date: 2026-10-11
decision-makers: Repository owner
---

# Close the follow-ups phase 6 left: test the union constructors, remove dead code, and draft the release notes

## Context and Problem Statement

[0011-PLAN-engine-tests.md](0011-PLAN-engine-tests.md) closed phase 6 of
[0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md) on `85ef7a8` and named three items it did not do.
Each needs a choice before work starts.

1. **The union constructors are barely tested from the root.** `helpers_gen.go` holds 11 constructors for four unions:
   `PlanUpdateContent` (3), `RequestPermissionOutcome` (2), `SessionConfigOption` (2), and `UnstableNesSuggestion` (4). The root tests
   cover 2 of its 11 statements (18.2%), through `json_parity_test.go`, which calls the two `RequestPermissionOutcome` constructors. The
   engine cases test the emitter that writes constructors (`accept-declared-discriminator`, `accept-null-member`), not these ones. A
   constructor sets its variant's own required keys and the discriminator; keys the union requires of every variant stay at their zero
   value. For example, `NewSessionConfigOptionBoolean(true)` leaves the required `id` and `name` of `SessionConfigOptionBoolean` empty.
   Nothing checks that each constructor sets the variant its name says, with the discriminator the schema gives.
2. **`util.SanitizeComment` is dead.** It is defined in `cmd/generate/internal/util/util.go`, marked deprecated, and neither called nor
   tested. The 0011-MADR lists it under F13, dead and stale code.
3. **The next release has no notes.** The latest tag is `v0.14.0` (`3588f8a`, 2026-10-10); 27 commits follow it on `main`, the work of
   [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md) and of 0011's phases 5 and 6. The exported API, as
   phase 6's listing program prints it, grew from 2181 to 2308 lines: 25 lines removed and 152 added. The records describe several changes to
   what the SDK sends or accepts. `RELEASING.md` puts release notes in the GitHub release, and asks for "any change to what the SDK sends or
   accepts on the wire". The repository keeps no changelog file.

## Decision Drivers

* A test should stay in step with the schema: a constructor added or removed by a schema change should be tested without editing a list by
  hand.
* No new dependency, and no change to the exported API or the wire.
* Release notes must be checked against the records and the API listing before a tag, and must be reviewable in a commit.
* The process the repository already has (records, `RELEASING.md`) is preferred to a new kind of file.

## Considered Options

For the constructors:

* C1. The registry `schema_gen_test.go` lists each constructor, called with zero arguments, and a root test checks each one.
* C2. A hand-written root test calls each of the 11 constructors.
* C3. Leave them as they are.

For `SanitizeComment`:

* S1. Delete it.
* S2. Keep it, deprecated.

For the release notes:

* R1. A report, `docs/reports/0013-REPORT-release-notes-v0-15-0.md`, drafts the notes. Its text goes into the GitHub release when `v0.15.0`
  is tagged.
* R2. A changelog file at the root.
* R3. No committed draft; the owner writes the notes at release time.

## Decision Outcome

Chosen options: "C1", "S1", and "R1", the owner's choice of 2026-10-11.

* C1, because the registry is generated from the same model as the constructors, so every constructor is listed and tested as soon as the
  schema has it; the test needs no knowledge of a constructor's arguments, since it checks the variant and the discriminator, which do not
  depend on them.
* S1, because the function has no caller, and the comment formatting it once did is `FormatDocComment`'s.
* R1, because a report is the repository's record of an observation that decides nothing, it is checked by `make check-records` and
  `make markdownlint`, and `RELEASING.md` already sends the text to the GitHub release; the version, `v0.15.0`, is the next minor after
  `v0.14.0`, which the owner confirms at tag time.

### Consequences

* Good, because every union constructor runs from the root, and a constructor that sets the wrong variant or discriminator fails a test.
* Good, because the dead function goes, and the release has notes the records support, reviewed before the tag.
* Neutral, because the constructor test checks the variant and the discriminator, not the zero values of the keys a union shares; whether a
  constructor should take those keys is not decided here.
* Bad, because `schema_gen_test.go` grows by a list, and the report must be brought up to date if more changes land before the tag.

### Confirmation

[0013-PLAN-close-phase-6-follow-ups.md](0013-PLAN-close-phase-6-follow-ups.md) records it: `helpers_gen.go` at 100% from the root tests,
the new test seen failing on planted faults, `SanitizeComment` gone with the build and tests passing, and each statement of the report traced
to a record, a commit, or the API listing.

## Pros and Cons of the Options

### C1. Registry list and a schema-driven test

* Good, because a schema change that adds or removes a constructor needs no hand edit of the test.
* Good, because the emitter's registry already lists types, unions, and methods for the conformance test, and is itself tested.
* Bad, because the registry emitter grows, and zero arguments do not show that a constructor puts each argument in its field.

### C2. A hand-written test

* Good, because each constructor's arguments can be given values and checked in their fields.
* Bad, because a schema change that adds a constructor goes untested until the list is edited, and one that removes one breaks the build.

### C3. Leave them

* Good, because nothing changes.
* Bad, because 9 of the 11 constructors stay untested from the root.

### S1. Delete `SanitizeComment`

* Good, because dead code goes.
* Neutral, because it is in an internal package, so no code outside the generator can call it.

### S2. Keep it

* Bad, because it stays dead, and mutation and coverage runs count it.

### R1. A report

* Good, because it is reviewed in a commit and checked by the record gates, and fits the existing process.
* Neutral, because its text is copied into the GitHub release by hand.

### R2. A changelog file

* Good, because consumers find a changelog where they look for one.
* Bad, because it is a new kind of file, beside the GitHub release that `RELEASING.md` names, and starts with one entry.

### R3. No draft

* Good, because nothing is written now.
* Bad, because the notes for 27 commits and the API changes are written at tag time, from memory, unchecked.

## More Information

The exported API was listed with phase 6's listing program on `git archive v0.14.0` and on `85ef7a8`. The 0011-PLAN's step 7 record lists
step 7's API changes; the 0011-MADR's "Found in step 7", "step 8", "step 9", and "step 12" sections and the 0012-MADR's amendments record the
wire changes.
