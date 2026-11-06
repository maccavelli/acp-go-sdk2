---
status: in-progress
date: 2026-11-05
associated-madr: "0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md"
---

# Implement 0050: the ids authenticate accepts, their refusal, the unknown entry's id, and the prompt result's doc

Associated MADR: [0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md](0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md)

## Goal

Ship the MADR's O1 as `v1.34.0`. At the end of it:

* a client takes the ids `Authenticate` will send from `AuthenticateMethodIDs`, which reads the same generated list as the
  gate;
* a refused id is answered with the ids that are accepted;
* an `Unknown` entry without a string `"id"` no longer advertises `"unknown"`;
* `PromptResponse`'s godoc says a successful turn sets `StopReason`;
* the documentation names S8's sentinel wherever it names S8's sibling.

Then issue #14 is answered, on the owner's ask.

## Scope

**Preconditions.**

* The MADR's questions Q1 to Q4 are answered: (a) each, on 2026-11-05, the recommended answers that this specification
  is written for. The MADR is `accepted`.
* This PLAN is approved by the owner before phase 1 begins.
* The close of [0049-PLAN-runtime-fixes.md](0049-PLAN-runtime-fixes.md) is committed, as `27f0cc5`. The base is that
  commit, whose code is `b4ee2d4`'s, the tree the MADR assessed.

**Files changed.** Phase by phase:

| Phase | Files |
| :--- | :--- |
| 1, the generator (D1, D2, D4) | `cmd/generate/internal/model/gates.go`, `gates_test.go`; `cmd/generate/internal/emit/gates.go`, `decode.go`; `cmd/generate/testdata/engine/accept-advertised-auth-ids.txtar` (new); `types_gen.go` (regenerated); `gates_surface_test.go` |
| 2, the helpers (D3, D6) | `helpers.go`; `auth_method_ids_test.go` (new); `example_auth_test.go` (new); `doc.go`; `docs_test.go`; `README.md`; `docs/architecture.md` |
| 3, the prompt result (D5, D7) | `cmd/generate/internal/model/acp.go`; `types_gen.go` (regenerated); `docs_test.go`; `acp_test.go` |
| 4, the close (D9) | `docs/reports/0050-REPORT-release-notes-v1-34-0.md` (new); this PLAN; the MADR; `docs/README.md` |

Each phase also updates this PLAN's execution record and its row in `docs/README.md`.

**Not changed:** S6 and S8 as rules; `go.mod`, `go.sum` and `cmd/generate/go.mod`; the schema; `.github/`; every exported
signature; what is sent on the wire, except F5's `"unknown"`, which is now refused.

**Out of scope:** the MADR's D8. That covers the prompt constructors, `AuthenticateAdvertised`, a `-32603` hint, a doc note
on `Agent.Prompt`, and the consumers' own changes.

## Implementation Steps

Every new check is first seen to fail on a planted break in a scratch copy. A plant is never made in the tree.

* **In the root module,** a plant that edits a hand-written file runs through `tools/plants.py`.
* **In the generator,** a plant needs a regeneration, or runs in the `cmd/generate` module, which `tools/plants.py`'s
  root-relative `go test` cannot reach. Such a plant runs by a scratch script in a copy, as the tooling PLAN's phase 5
  did. The script applies the edit, runs `cd cmd/generate && go run .` when the root's tests are to see it, then runs the
  named tests.
* **`TestEngineBehaviour`** runs whole in such a plant, not filtered to a subtest. Filtered, it skips the subtest that
  generates ([0049-PLAN-tooling-fixes.md](0049-PLAN-tooling-fixes.md), phase 5).

### Phase 1: the generator, D1, D2 and D4

1. **The model.**
   * `GateCheck` (`cmd/generate/internal/model/gates.go:39-73`) gains `Terminal string`. For a `GateCheckAdvertisedAuth`,
     it is the Go field of the variant that the same property's `GateCheckTerminalAuth` names, or empty when the gated
     type has no terminal check.
   * It is set where a gated type's checks are collected, after both checks exist: the advertised check of field `F` takes
     `Variant` from the terminal check of the same `Field` and `List`.
   * Both checks read the same union, so the variant is always one of `Advertised`, and no new refusal is needed.
   * The pin on `AuthenticateRequest` in `checkCommittedGates` (`gates_test.go:244-252`), which
     `protocol_test.go:513` runs over the committed schema, gains `a[1].Terminal != "Terminal"`.
2. **The string reader, D1.** `cmd/generate/internal/emit/decode.go` gains a helper, `unknownString(b json.RawMessage,
   key string) (string, bool)`:
   * it decodes `b` and folds `key` as `unknownKind` does;
   * it reports `(s, true)` when the value is a JSON string, the empty string included;
   * it reports `("", false)` otherwise, and for `b` that does not decode.

   It is a `helperUnknownString` constant beside `helperUnknownKind` (`decode.go:25-28`), emitted when used
   (`decode.go:58-68`). `emitUnknownKind` (`decode.go:482-507`) is rewritten to call it, keeping its `"unknown"` fallback
   for a value that is absent, empty or not a string, so `Kind()`'s results do not change. `markGateHelpers`
   (`emit/gates.go:50-59`) marks it for an advertised check with a `RawKey`.
3. **The list, D2.** For each `GateCheckAdvertisedAuth`, `emit/gates.go` emits one unexported function before the gates,
   named `accepted` + gated type + field + `s`. For ACP that is `acceptedAuthenticateRequestMethodIds(list []AuthMethod)
   []AuthMethodId`. It works in two passes over `list`:
   * **The terminal pass,** when `Terminal` is set, gathers every id a `Terminal` entry holds.
   * **The second pass,** in order, takes from each entry the id of the advertised variant it sets other than `Terminal`.
     Failing that, it takes the `Unknown` entry's id, through `unknownString(a.Unknown, RawKey)` when `RawKey` is set and
     the read reports a string. It appends that id unless the terminal pass gathered it, or it is already appended.
   * It returns nil when nothing is appended.
4. **The gate and its text, D4.** `advertisedGate` (`emit/gates.go:154-181`) becomes:

   ```go
   if ids := acceptedAuthenticateRequestMethodIds(n.Response.AuthMethods); !slices.Contains(ids, v.MethodId) {
       return fmt.Errorf("%s: methodId %q: %w; %s accepts %s", method, v.MethodId, ErrUnadvertisedAuthMethod, method, quotedIDs(ids))
   }
   ```

   * `quotedIDs[T ~string](ids []T) string` is a generated helper, emitted when used. It gives `none` for an empty list, or
     each id `%q`-quoted, joined by `", "`.
   * The terminal check (`emit/gates.go:73-77`) is unchanged and stays first.
   * The comment above `advertisedGate` cites this PLAN, D1, D2 and D4.
5. **The engine case.** `cmd/generate/testdata/engine/accept-advertised-auth-ids.txtar`, new:
   * **Its files:** `meta.json` with `initialize` and `authenticate`; a `schema.json` holding ACP's `AuthenticateRequest`,
     `AuthMethodId` and `AuthMethod` shapes from `schema/schema.json` (a `terminal` variant by `type`, and the default
     `agent`), and an `InitializeResponse` with `authMethods`; and `behaviour_test.go`.
   * **The capability the table needs:** `ClientCapabilities` holds `auth.terminal`, as ACP's does. Once `authMethods`
     exists, the protocol table's terminal-offer rule (`cmd/generate/internal/model/acp.go:221`) applies, and it needs
     that capability path.
   * **What its behaviour test requires,** in-package:
     * the list over `[agent a, terminal t, agent t, Unknown {"type":"x","id":"u"}, Unknown {"type":"x"}, agent a]` is
       exactly `[a u]`;
     * the gate refuses `t` with `ErrTerminalAuthMethod`, and refuses `unknown` and `z` with `ErrUnadvertisedAuthMethod`
       and the text `…; authenticate accepts "a", "u"`;
     * over an empty list, the text ends `accepts none`.
   * **`want.api`** is written by `cd cmd/generate && go test . -run '^TestEngineCases$/accept-advertised-auth-ids' -update`,
     then read in full.
6. **Regenerate:** `cd cmd/generate && go run .`. Then `git diff --stat` must show `types_gen.go` alone.
   * Its diff is read in full. It is the new `unknownString`, `quotedIDs` and `acceptedAuthenticateRequestMethodIds`,
     `unknownKind`'s new body, and `(*AuthenticateRequest).gates`' new advertised check. Nothing else changes.
7. **The root tests,** `gates_surface_test.go`:
   * **`TestAuthenticateUnadvertisedMethod`'s exact text** (`:146`) becomes D4's, for its answer of `a`, `t` and `f`:
     `…; authenticate accepts "a", "f"`.
   * **`TestAuthenticateUnknownEntryID`,** new, is P4 as a test. For each of `{"type":"future","name":"F"}`,
     `{"type":"future","id":"","name":"F"}` and `{"type":"future","id":7,"name":"F"}`, `methodId: "unknown"`:
     * is refused by `AllowsValue`;
     * is refused by `Authenticate`, with nothing written;
     * is answered `-32602` when received, with no handler run.
   * **The same test's accepted cases:** an entry `{"type":"future","id":"","name":"F"}` makes `methodId: ""` accepted.
     `{"type":"future","ID":"f","name":"F"}` makes `f` accepted.
   * **`TestAuthenticateAcceptsNone`,** new: an answer of one terminal method refuses an agent id with the text ending
     `authenticate accepts none`.
8. **Seen to fail,** by the scratch script, each plant regenerating in its copy and then running the named tests, with
   `TestEngineBehaviour` whole:
   * the list reading `Unknown` through `unknownKind` again: `TestAuthenticateUnknownEntryID` fails, and so does the
     engine case;
   * the terminal pass removed: the engine case's `[a u]` fails;
   * the text without `; %s accepts %s`: `TestAuthenticateUnadvertisedMethod` and the engine case fail;
   * `Terminal` left unset in the model: the model test fails.
9. **Checks:**
   * `cd cmd/generate && go test -shuffle=on ./...`;
   * `go test -race -count=1 -shuffle=on .`;
   * `tools/gates.py regen` reports the generated files unchanged;
   * `make pre-add-check FILES="<the phase's files>"`.

   Then stage the files for the owner.

### Phase 2: the helpers and the documentation, D3 and D6

1. **`helpers.go`:**
   * **`func (a AuthMethod) ID() AuthMethodId`,** after `Kind()` (`helpers.go:418-433`): `Terminal.Id`, `Agent.Id`, or
     `Unknown`'s `"id"` through `unknownString`, `""` when that reports no string; `""` for the zero value. Its godoc
     cites the MADR's D3 and says an empty `ID()` does not mean the entry is not authenticatable: use
     `AuthenticateMethodIDs`.
   * **`func AuthenticateMethodIDs(methods []AuthMethod) []AuthMethodId`,** after `OfferedAuthMethods`
     (`helpers.go:740-756`): it returns `acceptedAuthenticateRequestMethodIds(methods)`, and is not copied. Its godoc
     says:
     * it reads an `initialize` answer's `AuthMethods`, as `Initialize` returns it or as `Negotiated()` records it;
     * an id it returns is one `Authenticate` sends, and every other id is refused before anything is sent: a terminal
       one with `ErrTerminalAuthMethod`, any other with `ErrUnadvertisedAuthMethod`;
     * entries of a type this SDK does not know are included by their string `"id"`, and `Kind()` filters them;
     * it cites the MADR's D2 and D3.
2. **Tests,** `auth_method_ids_test.go`, new:
   * **`TestAuthMethodID`:** each variant; `Unknown` with `"id"`, with `"ID"`, with `""`, without one, and with `7`; and the
     zero value.
   * **`TestAuthenticateMethodIDsIsTheGate`:** over each of five answers, every id returned passes `AllowsValue`. Every
     other id of the answer (`ID()` of each entry), and `"unknown"` and `"stranger"`, is refused with `ErrTerminalAuthMethod`
     or `ErrUnadvertisedAuthMethod`. The five answers:
     * P2's;
     * P3's, the id that is both terminal and agent;
     * one with P4's entry;
     * one with a duplicate agent id;
     * the empty answer.
   * **The order:** the list follows the answer's order, with each id once.
3. **`example_auth_test.go`,** new, package `acp_test`: `ExampleAuthenticateMethodIDs` builds an answer of `openai` and
   `grok`, agent methods, and `grok-login`, terminal. It picks a pin when the list holds it, and the first id otherwise. Its
   `// Output:` shows `[openai grok]`, the pin `grok` taken, and the stale pin `grok-login` falling back to `openai`.
4. **Documentation:**
   * **`AllowsValue`'s godoc** (`helpers.go:1020-1031`) names `ErrUnadvertisedAuthMethod` beside `ErrTerminalAuthMethod`.
   * **`doc.go:36-38`'s sentinel list** names `[ErrUnadvertisedAuthMethod]`.
   * **`docs_test.go`** gains `TestPackageDocNamesAuthSentinels`: `doc.go`'s comment names both `ErrTerminalAuthMethod`
     and `ErrUnadvertisedAuthMethod`.
   * **`README.md:611-617`** says:
     * an `Unknown` entry counts by its `"id"` when that is a string (D1);
     * the refusal names the ids `authenticate` accepts (D4);
     * `acp.AuthenticateMethodIDs(resp.AuthMethods)` lists them, and `AuthMethod.ID()` reads one entry's id.
   * **`README.md`'s client section,** after the `Implement the acp.Client interface` bullet (`:133-138`), gains one
     bullet: take an `authenticate` id from `acp.AuthenticateMethodIDs`, since the SDK refuses any other before sending.
   * **`docs/architecture.md:81-86`'s list** of what `helpers.go` holds names `ID()` and `AuthenticateMethodIDs`.
5. **Seen to fail,** with `tools/plants.py`:
   * `ID()` returning `"unknown"` for an `Unknown` entry without a string id: `TestAuthMethodID` fails;
   * `AuthenticateMethodIDs` returning every entry's `ID()` unfiltered: `TestAuthenticateMethodIDsIsTheGate` fails;
   * the example's fallback taking the last id: `ExampleAuthenticateMethodIDs` fails;
   * `[ErrUnadvertisedAuthMethod]` removed from `doc.go`: `TestPackageDocNamesAuthSentinels` fails.
6. **Checks:**
   * `go test -race -count=1 -shuffle=on ./...`;
   * `make api`, "against v1.33.1: 0 incompatible, 2 compatible", naming `AuthMethod.ID` and `AuthenticateMethodIDs`;
   * `make pre-add-check FILES="<the phase's files>"`;
   * `make check-records`, `make markdownlint` and `make disclosure-check`.

   Then stage the files.

### Phase 3: the prompt result, D5 and D7

1. **The doc note.** `cmd/generate/internal/model/acp.go`'s `DocNotes` (`:155-169`) gain a `PromptResponse` entry, its
   lines wrapped below 80 columns:

   ```text
   StopReason is required and has no default: its zero value, "", is not a
   StopReason, and an agent whose Prompt returns it answers -32603 instead
   of sending it. A successful turn sets one of the constants, such as
   StopReasonEndTurn (0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md, D5).
   ```

2. **Regenerate:** `cd cmd/generate && go run .`. The diff of `types_gen.go` is `PromptResponse`'s godoc alone.
3. **`docs_test.go`** gains `TestPromptResponseDocNamesTheStopReasonRule`: `PromptResponse`'s godoc, read with
   `go/doc` from the package, holds `-32603` and `StopReasonEndTurn`.
4. **The stubs,** `acp_test.go:200-205`, `:409-411` and `:1340-1342`, return `PromptResponse{StopReason:
   StopReasonEndTurn}, nil`. `TestClosedEnumsRefusedOnSend` keeps its planted zero result.
5. **Seen to fail,** by the scratch script: the doc note removed and the copy regenerated, so
   `TestPromptResponseDocNamesTheStopReasonRule` fails.
6. **Checks:**
   * `go test -race -count=1 -shuffle=on ./...`;
   * `tools/gates.py regen`;
   * `make pre-add-check FILES="<the phase's files>"`.

   Then stage the files.

### Phase 4: the close and the release, D9

1. **The gates:** `tools/gates.py --all`, with the Linux and Windows hosts named per command. Then a table of each
   decision, D1 to D7, with the test that shows it.
2. **CI:** after the owner's push, CI on the head, read by `tools/watch_ci.py`.
3. **The release,** on the owner's ask:
   * `docs/reports/0050-REPORT-release-notes-v1-34-0.md` leads with D4 and D3, then D1 as a behaviour change, then D5 and
     D6. Each statement names its commit.
   * Before the push: `make push-guard`, after a fetch.
   * `tools/release_body.py v1.34.0 <rev>`.
   * The owner tags `v1.34.0` and creates the release.
   * After the tag: `tools/consumer.py v1.34.0` and `tools/publication.py v1.34.0 <rev> <the body>`.
4. **The issue reply,** posted only on the owner's ask, with `gh issue comment 14 --body-file <file>`. Its draft goes in this
   PLAN's execution record first. Its content:
   * what shipped in `v1.34.0`, with the helper and the new refusal text;
   * the corrections to the issue's table (MADR F2): `refusals_test.go` and `connection_overflow_block_test.go`, and the
     stubs reached by `TestConcurrentUse` alone;
   * why the prompt constructors were declined (MADR O2);
   * the consumer facts of F10: magic-cli-remote's pin and key-method paths, and gobble-cli's two tests that expect
     `-32602`.

   Closing the issue is the owner's choice.
5. **Status:** this PLAN and its row are marked `complete`.

## Verification

* **D1:** `TestAuthenticateUnknownEntryID` passes, and failed with the list reading `Unknown` through `unknownKind`.
* **D2:** `TestAuthenticateMethodIDsIsTheGate` and the engine case `accept-advertised-auth-ids` pass, and each failed on
  its plant.
* **D3:** `TestAuthMethodID` and `ExampleAuthenticateMethodIDs` pass, and each failed on its plant.
* **D4:** the exact texts, with ids and with none, in `TestAuthenticateUnadvertisedMethod` and `TestAuthenticateAcceptsNone`,
  on send and as received.
* **D5:** `TestPromptResponseDocNamesTheStopReasonRule`.
* **D6:** `TestPackageDocNamesAuthSentinels`, and the README and architecture text read in full.
* **D7:** the suite passes, and the root's only success-path `PromptResponse{}` is `TestClosedEnumsRefusedOnSend`'s.
* **`make api`** against `v1.33.1`: "0 incompatible, 2 compatible".
* **The gates and CI:** `tools/gates.py regen` is clean after each phase. `tools/gates.py --all`, 18 of 18, and CI on the
  head and on the tag, pass.
* **The benchmarks:** `go test -run '^$' -bench 'PromptRoundTrip|SessionUpdateNotify|SessionNotificationUnmarshal'
  -benchmem -count=5 .` (`bench_test.go`) before phase 1 and after phase 3, with allocations per operation unchanged. The
  changed gate runs only for `authenticate`.

## Rollout and Rollback

* **Rollout:** a minor, `v1.34.0`. What a consumer meets:
  * **The new helpers:** `AuthMethod.ID()` and `AuthenticateMethodIDs`.
  * **A longer refusal text:** `ErrUnadvertisedAuthMethod`'s error, sent and received, ends with the accepted ids.
    `errors.Is` is unchanged. Code that compares the whole string, as gobble-cli's tests do with codes, sees the longer
    text.
  * **`"unknown"` refused:** `methodId: "unknown"` against an `Unknown` entry without a string `"id"` is now refused, sent
    or received.
  * **The godoc:** `PromptResponse`'s godoc states the rule.
* **Rollback:** each phase reverts on its own, by reverting its commit, before the tag. After `v1.34.0` is published, the
  two names are a `v1` promise. A rollback then removes nothing exported; it only reverts behaviour, D1 or D4, in a patch.

## Execution record

The owner committed the pair as `4037547` and said "committed, approved. proceed".

### Phase 1: the generator, D1, D2 and D4, 2026-11-05

**Before:** the benchmark baseline, `go test -run '^$' -bench 'PromptRoundTrip|SessionUpdateNotify|SessionNotificationUnmarshal'
-benchmem -count=5 .` on `4037547`, kept for phase 3's comparison.

**What it did:**

* **The model,** step 1.1:
  * `GateCheck` gained `Terminal`. `linkTerminal`, called in `resolveGates` once each type's own checks are collected, sets
    it on an advertised check from the terminal check of the same `Field` and `List`.
  * The pin on `AuthenticateRequest` requires `Terminal: "Terminal"`. That pin moved out of `checkCommittedGates` into a
    helper of its own, `checkAuthenticateGates`, which it calls. The added condition made `checkCommittedGates`' cyclomatic
    complexity 41, over `gocyclo`'s 40 (`make pre-add-check`: "cyclomatic complexity 41 of func `checkCommittedGates` is
    high (> 40)"). The checks are the same, and the plant below fails through the helper.
* **The string reader,** step 1.2:
  * `decode.go` gained `helperUnknownString` and `emitUnknownString`, which emits `unknownString(b, key) (string, bool)`.
    It reports `("", false)` for `b` that does not decode, and for a key that is absent, `null` or not a string.
  * `emitUnknownKind` now emits `unknownKind` reading through it, with the `"unknown"` fallback kept for an absent, empty or
    non-string value.
  * `emitDecodeHelpers` emits `unknownString` whenever `unknownKind` is emitted, and `quotedIDs` when it is used.
* **The list,** step 1.3: `emitAcceptedIDs` emits `acceptedAuthenticateRequestMethodIds`, named by `acceptedIDsName`. It
  makes two passes: the terminal ids first, then each entry's `Agent` id, or its `Unknown` string id. It appends an id not
  among the terminal ones and not already appended.
* **The gate,** step 1.4:
  * `advertisedGate` takes the list function's name, and refuses an id the list lacks with `"%s: methodId %q: %w; %s
    accepts %s"` and `quotedIDs(ids)`.
  * `quotedIDs[T ~string]` is emitted with a type parameter on the `ast.FuncType`. `markGateHelpers` marks it and
    `unknownString` for an advertised check, and `unknownKind` for any other check with a `RawKey`.
  * The comments cite the MADR's D1, D2 and D4.
* **The engine case,** step 1.5: `accept-advertised-auth-ids.txtar`.
  * It holds ACP's `AuthMethod`, `AuthMethodAgent`, `AuthMethodTerminal`, `AuthMethodId`, `AuthenticateRequest`,
    `AuthenticateResponse`, `AuthCapabilities` and `ProtocolVersion`, copied from `schema/schema.json` with their
    descriptions, `_meta` dropped. It also holds a `ClientCapabilities` of `auth`, an `AgentCapabilities` of `loadSession`,
    and the two `initialize` definitions with `authMethods`.
  * It also needed a `CancelRequestNotification`. Without one the model refused the case: "protocol method
    "$/cancel_request": no request or notification definition is linked to it by x-method and x-side". It takes the same
    shape as `accept-gates-unreachable`'s.
  * Its `want.api` was written by `go test . -run '^TestEngineCases$/^accept-advertised-auth-ids$' -update` and read in
    full: the union with `Unknown`, the gated `Authenticate`, and no new exported name.
  * Its behaviour test, `TestAcceptedAuthMethodIDs`, runs under `TestEngineBehaviour`: "--- PASS:
    TestEngineBehaviour/accept-advertised-auth-ids".
* **Regenerated,** step 1.6: `cd cmd/generate && go run .`. `git diff --stat` showed `types_gen.go` alone, 66 insertions and
  9 deletions, and its diff was read in full. It holds:
  * `unknownKind` through `unknownString`, then `unknownString` and `quotedIDs`;
  * `acceptedAuthenticateRequestMethodIds`, its comment wrapped below 80 columns after a first run printed it in one line;
  * the advertised check of `(*AuthenticateRequest).gates`.
* **The root tests,** step 1.7, `gates_surface_test.go`:
  * `TestAuthenticateUnadvertisedMethod` requires `; authenticate accepts "a", "f"`, sent and in the `-32602` received.
  * `TestAuthenticateUnknownEntryID` refuses `methodId: "unknown"` against an entry whose `"id"` is absent, `""`, `null`,
    `7` or an object: in `AllowsValue`, on send with nothing written, and as received. It accepts `""`, `"f"` by `"ID"`, and
    `"unknown"` itself, each when it is the entry's string id. `null` and an object are added to the PLAN's three.
  * `TestAuthenticateAcceptsNone` refuses against one terminal method with `…; authenticate accepts none`. It refuses an id
    that is both terminal and agent as terminal, and then names `"b"` alone as accepted.

**Seen to fail,** step 1.8, by `plants_p1.py`, a scratch script that copies the working tree for each plant, regenerates
where the root's tests must see the change, and runs `TestEngineBehaviour` whole. 7 of 7 runs failed as required:

```text
FAILED AS REQUIRED: the list reading Unknown through unknownKind again (. -run ^TestAuthenticateUnknownEntryID$ .): exit 1: gates_surface_test.go:218: {"type":"future","name":"F"}: AllowsValue(unknown) = <nil>, want ErrUnadvertisedAuthMethod
FAILED AS REQUIRED: the list reading Unknown through unknownKind again (cmd/generate -run ^TestEngineBehaviour$ .): exit 1: behaviour_test.go:36: accepted ids = ["a" "u" "unknown"], want [a u]
FAILED AS REQUIRED: the terminal pass removed (cmd/generate -run ^TestEngineBehaviour$ .): exit 1: behaviour_test.go:36: accepted ids = ["a" "t" "u"], want [a u]
FAILED AS REQUIRED: the terminal pass removed (. -run ^TestAuthenticateAcceptsNone$ .): exit 1: gates_surface_test.go:272: AllowsValue(z) = authenticate: methodId "z": acp: methodId names no auth method the agent advertised; authenticate accepts "t", "b", want the accepted ids to be b alone
FAILED AS REQUIRED: the text without the accepted ids (. -run ^TestAuthenticateUnadvertisedMethod$ .): exit 1: gates_surface_test.go:150: Authenticate(x) = authenticate: methodId "x": acp: methodId names no auth method the agent advertised, want ErrUnadvertisedAuthMethod
FAILED AS REQUIRED: the text without the accepted ids (cmd/generate -run ^TestEngineBehaviour$ .): exit 1: behaviour_test.go:48: unknown: authenticate: methodId "unknown": acp: methodId names no auth method the agent advertised, want ErrUnadvertisedAuthMethod naming a and u
FAILED AS REQUIRED: Terminal left unset in the model (cmd/generate ./internal/model/): exit 1: protocol_test.go:513: AuthenticateRequest: [{Kind:4 Field:MethodId …
```

The last plant was run again after the pin moved into `checkAuthenticateGates`, and failed the same way.

**Checks,** step 1.9:

* `cd cmd/generate && go test -count=1 -shuffle=on ./...`: `ok` for all seven packages.
* `go test -race -count=1 -shuffle=on .`: `ok`, 16.3 s.
* `tools/gates.py regen`: "ok regen (exit 0)", the generated files unchanged.
* `make pre-add-check` with the phase's seven files:
  * first, exit 2, `gocyclo` on `checkCommittedGates`, as above;
  * then exit 0, at a load of 27 to 47: "6 file(s) gofmt-clean; both modules linted, vetted and tested (-race -shuffle=on,
    uncached), tidy, govulncheck."

No deviation: the pin's move and the case's `CancelRequestNotification` are details the steps did not name, inside the
phase's files. Staged for the owner, who commits. Phase 2 begins on the owner's word.

### Phase 2: the helpers and the documentation, D3 and D6, 2026-11-05

The owner committed phase 1 as `ba14c67` and said "committed proceed".

**What it did:**

* **`helpers.go`,** step 2.1:
  * `AuthMethod.ID()`, after `Kind()`, returns `Terminal.Id`, `Agent.Id`, or `Unknown`'s `"id"` through `unknownString`
    when that reports a string, and `""` otherwise.
  * `AuthenticateMethodIDs(methods)`, after `OfferedAuthMethods`, returns `acceptedAuthenticateRequestMethodIds(methods)`.
  * Both godocs cite the MADR's D2 and D3, and say what step 2.1 asks.
* **The tests,** step 2.2, `auth_method_ids_test.go`:
  * `TestAuthMethodID`, nine cases: an agent; a terminal; `Unknown` with `"id"`, with `"ID"`, with `""`, without one, with
    `null` and with `7`; and the zero value.
  * `TestAuthenticateMethodIDsIsTheGate`, five answers: P2's; P3's id that is both terminal and agent; `Unknown` entries
    with and without an id; a duplicate agent id; and none.
  * For each answer, the test checks the list, its order and each id once. Every listed id passes `AllowsValue`; every
    other entry's `ID()`, and `"unknown"` and `"stranger"`, is refused as terminal or as unadvertised.
* **The example,** step 2.3: `example_auth_test.go`, `ExampleAuthenticateMethodIDs`, with the output `[openai grok]`, `grok`
  and `openai`.
* **Documentation,** step 2.4:
  * `AllowsValue`'s godoc names `ErrUnadvertisedAuthMethod` and that its text names `AuthenticateMethodIDs`' ids.
  * `doc.go`'s sentinel list names `[ErrUnadvertisedAuthMethod]`, its paragraph rewrapped.
  * `docs_test.go`'s `TestPackageDocNamesAuthSentinels` reads `doc.go`'s package comment with `go/parser`.
  * `README.md`'s authenticate passage gives the refusal's text, D1's rule, and the two helpers. Its client section gains
    the bullet.
  * `docs/architecture.md`:
    * the list of what `helpers.go` holds names the two helpers;
    * the gates passage names `unknownString`, `acceptedAuthenticateRequestMethodIds` and `quotedIDs`;
    * the decoders passage says `unknownKind` reads through `unknownString`;
    * the tests list names `auth_method_ids_test.go`, the new `gates_surface_test.go` checks, and the engine case
      `accept-advertised-auth-ids`.

    The last three are phase 1's generator work, which phase 1 did not document.

**Seen to fail,** step 2.5, by `tools/plants.py`, 4/4:

```text
FAILED AS REQUIRED: ID() returning unknownKind's fallback for an entry without a string id (^TestAuthMethodID$): exit 1: auth_method_ids_test.go:31: unknown with an empty id: ID() = "unknown", want ""
FAILED AS REQUIRED: AuthenticateMethodIDs returning every entry's ID unfiltered (^TestAuthenticateMethodIDsIsTheGate$): exit 1: auth_method_ids_test.go:61: agents and a terminal: AuthenticateMethodIDs = ["openai" "grok" "grok-login"], want ["openai" "grok"]
FAILED AS REQUIRED: the example's fallback taking the last id (^ExampleAuthenticateMethodIDs$): exit 1: --- FAIL: ExampleAuthenticateMethodIDs (0.00s)
FAILED AS REQUIRED: ErrUnadvertisedAuthMethod removed from the package doc (^TestPackageDocNamesAuthSentinels$): exit 1: docs_test.go:360: the package doc does not name [ErrUnadvertisedAuthMethod]
```

**Checks,** step 2.6:

* `go test -race -count=1 -shuffle=on ./...`: `ok`, all nine packages with tests.
* `make api`: "compatible: AuthMethod.ID: added", "compatible: AuthenticateMethodIDs: added", "against v1.33.1: 0
  incompatible, 2 compatible".
* `make pre-add-check` with the phase's seven files: exit 0, at a load of 11 to 13, "5 file(s) gofmt-clean; both modules
  linted, vetted and tested (-race -shuffle=on, uncached), tidy, govulncheck."
* `make check-records`, `make markdownlint` and `make disclosure-check`: exit 0 each.

No deviation. Staged for the owner, who commits. Phase 3 begins on the owner's word.
