# Release notes for v0.17.1, against v0.17.0

Observed 2026-10-13 on `3a9b5c4`, for
[0018-PLAN-start-side-connections-after-assignment.md](../decisions/0018-PLAN-start-side-connections-after-assignment.md), step 6.
This report drafts the text of the GitHub release for `v0.17.1`, as `RELEASING.md` describes it. It decides nothing. Each statement names
its source: a record, a commit, GitHub issue #3, or the exported API listing of the 0013 report's listing program, run on `git archive
v0.17.0` and on the tree at `3a9b5c4` (2411 lines each). Bring it up to date if more changes land before the tag. The version is the next
patch, `v0.17.1`; the owner confirms it at the tag.

The section "Release text" is what goes into the GitHub release.

## Release text

### v0.17.1

A patch release that fixes one defect; nothing else changes. The ACP schema is still `1.25.0` (`git diff v0.17.0 -- schema/` is empty),
and the exported API is unchanged. Upgrade from `v0.16.0` or `v0.17.0` needs no code change. (0018-MADR; issue #3.)

#### Fixes

- **A message that arrives at once no longer races the side connection's constructor.** `NewAgentSideConnection` and
  `NewClientSideConnection` started reading before they had stored their connection, so a message the peer had already written, such as
  the `initialize` an editor or a stdio client sends at once, could be handled while that field was unset. Under `-race` this was a data
  race; when the read came first, the handler dereferenced a nil `*Connection` and the process panicked. Each constructor now assigns
  every field before reading starts. Both sides were affected: an agent by an immediate `initialize`, a client by any request an agent
  wrote before the constructor returned. Affected versions: `v0.16.0` and `v0.17.0`. (`3a9b5c4`; 0018-MADR; issue #3.)
- `NewConnection` behaves as before. Its documentation now says that reading starts before it returns, so a handler must not read a
  variable assigned from its result: code that builds its own connection that way has the same race. (`3a9b5c4`; 0018-MADR, outcome 2.)

#### For the generator

- The side connections' constructors are emitted as: assign every field, `x.conn = buildConnection(...)`, `x.conn.start()`, `return x`.
  The generator's test pins that order. (`3a9b5c4`; 0018-PLAN, steps 2 to 4.)

## Sources not in the release text

- The commits since `v0.17.0`: `b5f13fb` (records only) and `3a9b5c4`, the only one that changes Go code, in the SDK
  (`git log v0.17.0..HEAD -- '*.go' ':!cmd/generate'`) and in the generator (`git log v0.17.0..HEAD -- cmd/generate`).
- The API listing difference, classified: changed, none; removed, none; added, none. `buildConnection` and `(*Connection).start` are
  unexported.
- The reproduction, the test `TestConstructorAssignsBeforeReading`, the planted breaks it and the generator's test were seen to fail on,
  and the Windows run are in the PLAN's execution record, steps 2 to 4. The new test detects the old order only under `-race`; CI runs
  `-race` on Linux.
