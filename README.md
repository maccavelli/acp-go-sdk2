<a href="https://agentclientprotocol.com/" >
  <img alt="Agent Client Protocol" src="https://zed.dev/img/acp/banner-dark.webp">
</a>

# ACP Go SDK

Go library for the Agent Client Protocol (ACP) - a standardized communication protocol
between code editors and AI‑powered coding agents.

Learn more about the protocol itself at <https://agentclientprotocol.com>.

## Installation

<!-- `$ printf 'go get github.com/maccavelli/acp-go-sdk2\n'` as bash -->

```bash
go get github.com/maccavelli/acp-go-sdk2
```

It requires Go 1.26.10 or newer, the `go` line of `go.mod`; CI tests Go 1.26.10
and 1.27.3.

## Get Started

### Understand the Protocol

Start by reading the [official ACP documentation](https://agentclientprotocol.com)
to understand the core concepts and protocol specification.

### A first connection

This is `ExampleNewClientSideConnection`, which `go test` runs: a client and an
agent in one process over two pipes. `demoAgent` and `demoClient` are the
example `Agent` and `Client` in `example_agent_test.go` and
`example_client_test.go`. A real client starts the agent's process and
passes its stdin and stdout instead of the pipes, as `example/client` does.

<!-- quickstart: ExampleNewClientSideConnection -->
```go
// Each side writes to one pipe and reads the other.
toAgent, fromClient := io.Pipe()
toClient, fromAgent := io.Pipe()
agent := acp.NewAgentSideConnection(&demoAgent{}, fromAgent, toAgent)
client := acp.NewClientSideConnection(&demoClient{}, fromClient, toClient)

resp, err := client.Initialize(context.Background(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
if err != nil {
    fmt.Println("initialize:", err)
    return
}
fmt.Println("protocol version:", resp.ProtocolVersion)

// Closing the client ends the agent's input, so the agent ends too.
if err := client.Close(); err != nil {
    fmt.Println("close:", err)
}
<-agent.Done()
if err := agent.Close(); err != nil {
    fmt.Println("close:", err)
}
// Output:
// protocol version: 1
```
<!-- /quickstart -->

### Try the Examples

The [examples directory](https://github.com/maccavelli/acp-go-sdk2/tree/main/example)
contains simple implementations of both Agents and Clients in Go.
You can run them from your terminal or connect to external ACP agents.

- `go run ./example/agent` starts a minimal ACP agent over stdio.
- `go run ./example/claude-code` bridges to Claude Code through its ACP adapter,
  `@agentclientprotocol/claude-agent-acp`, which it runs with `npx`.
- `go run ./example/client` starts the example agent with `go run` (from a
  source checkout) or the agent command given as its arguments, sends one
  prompt, and prints the streamed turn and its stop reason.
- `go run ./example/gemini` bridges to the Gemini CLI in ACP mode, `gemini --acp`
  (flags: -model, -sandbox, -yolo, -debug, -gemini /path/to/gemini).
- `go run ./example/remote serve -addr 127.0.0.1:7331` serves an echo agent over
  the network, and `go run ./example/remote dial http://127.0.0.1:7331/acp`
  (or `ws://…`) prompts it and loads the session again
  ([Remote agents](#remote-agents)).

The example clients advertise the file-system capability and not the terminal
one, and write only inside the session's working directory. You can watch the
interaction by running `go run ./example/client` locally, answering the
permission request with `1` or `2`.

### Explore the API

Browse the Go package docs on pkg.go.dev for detailed API documentation:

- <https://pkg.go.dev/github.com/maccavelli/acp-go-sdk2>

If you're building an [Agent](https://agentclientprotocol.com/protocol/overview#agent):

- Implement the `acp.Agent` interface (and optionally `acp.AgentLoader` for `session/load`).
  An agent that offers none of `session/list`, `session/delete`,
  `session/resume`, `session/close` and `logout` embeds `acp.AgentRefusals`,
  which answers each with method-not-found, and implements the other seven; an
  agent that implements some of the five implements all five instead.
- Create a connection with `acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)`.
  `acp.WithInboundRewrite(acp.ResolveStdioCommands(exec.LookPath))` resolves a
  client's bare MCP server command, such as `npx`, before the SDK refuses it
  as not absolute; `acp.ChainRewrites` combines it with rewrites of your own.
  Answer `initialize` with `acp.OfferedAuthMethods(req.ClientCapabilities,
  methods)` so a client without `auth.terminal` is not offered a terminal
  method, which the SDK would refuse to send.
- To serve over the network instead, mount `remote.NewHandler` on your
  `http.Server`, with `acp.ServeAgent` as its serve function
  ([Remote agents](#remote-agents)).
- Send updates and make client requests using the returned connection. An agent
  that keeps the connection for its handlers implements `acp.AgentConnAware`:
  the constructor calls its `SetAgentConnection` before it reads anything, so a
  handler always finds the connection set. The method only stores it.
  Assigning the returned connection after the constructor instead races a
  handler that is already running.
- `acp.PresentNotice(ctx, conn, sessionId, notice)` sends a notice the way
  the client can show it. A client that advertised `session.notices` gets the
  notice update. Any other client gets an agent message of the title, and of
  the description on a line after it, which becomes conversation history.
  `acp.PresentCompaction` does the same for a compaction's state, sending
  "Compaction" and its status, such as "Compaction completed", and its error,
  if any, as the fallback.
  `acp.PresentCompactionSummary` sends a summary chunk only to a client that
  advertised `session.compaction`. To send the dedicated update or nothing,
  use `SessionUpdate` with `acp.UpdateNotice` or `acp.UpdateCompaction`, which
  fail with a `*CapabilityError` without the capability.

If you're building a [Client](https://agentclientprotocol.com/protocol/overview#client):

- Implement the `acp.Client` interface. It includes the file-system and terminal
  methods, such as `ReadTextFile` and `CreateTerminal`. A client that offers
  none of the file-system, terminal and elicitation methods embeds
  `acp.ClientRefusals`, which answers each with method-not-found, and
  implements only `SessionUpdate` and `RequestPermission`; a client that
  implements some of them implements all of them instead.
- Package `clienthost` serves those methods for you. `clienthost.Terminals`
  runs each `terminal/create`'s command in a process group, or a job on
  Windows, keeps its output to `outputByteLimit` at a character boundary, and
  ends the processes it started on kill and release; its `Command` field
  builds the command, and is where your policy checks it. `clienthost.Files`
  reads and writes text files, by 1-based `line` and `limit`, answering a
  missing file `-32002`. Embed them beside `clienthost.Refusals`, which refuses
  the rest one level deeper, so that their methods win:

  ```go
  type client struct {
      clienthost.Terminals
      clienthost.Files
      clienthost.Refusals
  }
  ```

  Your own `ReadTextFile` or `WriteTextFile` wins over the embedded one, to
  check the path against the session's roots before calling it; `Close` ends
  every terminal
  ([0041-MADR-close-the-gaps-the-consumers-wait-on.md](docs/decisions/0041-MADR-close-the-gaps-the-consumers-wait-on.md)).
- Launch or connect to your Agent process (stdio), then create a connection with
  `acp.NewClientSideConnection(client, stdin, stdout)`. For an agent on the
  network, `remote.Dial` returns a `*remote.Conn` that is both streams:
  `acp.NewClientSideConnection(client, c, c)` ([Remote agents](#remote-agents)).
- Call `Initialize`, `NewSession`, and `Prompt` to run a turn and stream updates.
  A client that keeps its connection implements `acp.ClientConnAware`, whose
  `SetClientConnection` the constructor calls the same way.
- `acp.AllSessions(ctx, client, req)` ranges over every session
  `session/list` pages through, and stops with `acp.ErrRepeatedCursor` when
  the agent repeats a cursor. Helpers that take a side connection are
  functions, since a side connection's methods are its protocol methods.
- `acp.PickPermissionOption` chooses a permission option by its kind, never by
  its name, so an option named "Don't allow" is never taken as an allow.
- A client that advertises `session.compaction` keeps one
  `acp.CompactionTimeline` per session and feeds it every `SessionUpdate`,
  live or replayed by `session/load`. `Apply` handles the protocol's update
  rules:
  - a compaction first seen starts `in_progress`;
  - an absent patch keeps a field, a `null` one clears it, and a value
    replaces it;
  - chunks append, a concrete `summary` replaces them, and `[]` clears it;
  - an unknown status is kept as it arrived.

  `Get` returns a compaction. `Open` lists those not known to have ended, so
  they can be shown as unfinished when a turn ends without them.
- `acp.ToolCallTimeline` does the same for tool calls: a `tool_call` is the
  call, and a `tool_call_update` changes only the fields it carries, its
  `content` and `locations` replacing the collections, an empty one clearing
  them; an update for a call never seen starts it `pending`. `Get` and `Open`,
  the calls neither `completed` nor `failed`, read it back.

Helper constructors are provided to reduce boilerplate when working with union types:

- Content blocks: `acp.TextBlock`, `acp.ImageBlock`, `acp.AudioBlock`,
  `acp.ResourceLinkBlock`, `acp.ResourceBlock`.
- Tool content: `acp.ToolContent`, `acp.ToolDiffContent`, `acp.ToolTerminalRef`.
- Session updates: `acp.UpdateUserMessage`, `acp.UpdateAgentMessage` and
  `acp.UpdateAgentThought`, each with a `Text` form; `acp.UpdatePlan`;
  `acp.UpdateAvailableCommands`, `acp.UpdateCurrentMode`,
  `acp.UpdateConfigOptions`, `acp.UpdateSessionTitle`, `acp.UpdateUsage`,
  `acp.UpdateNotice`, `acp.UpdateCompaction`, `acp.UpdateCompactionSummary`
  and `acp.UpdateCompactionSummaryText`. Each takes the variant's required
  keys; optional fields are set on the variant it returns. `UpdateNotice` and
  the compaction updates are refused unless the client advertised
  `session.notices` or `session.compaction`. `SessionUpdate.Kind()` names the
  variant a value sends, or the `sessionUpdate` of an unknown one received;
  `ContentBlock`, `ToolCallContent`, `RequestPermissionOutcome`,
  `SessionConfigOption`, `AuthMethod`, `McpServer`,
  `CreateElicitationRequest`, `CreateElicitationResponse`, `MultiSelectItems`,
  `SetSessionConfigOptionRequest` and the unstable `PlanUpdateContent` have
  the same `Kind()`, reading `type`, `outcome`, `mode` or `action`. A variant
  sent with no discriminator is named as the schema names it: `agent`,
  `stdio`, `value_id` or `titled`.
- Tool calls: `acp.StartToolCall`, `acp.UpdateToolCall`, and their options.
  `rawInput` and `rawOutput` are raw JSON, so `acp.WithStartRawInput`,
  `acp.WithStartRawOutput`, `acp.WithUpdateRawInput`, `acp.WithUpdateRawOutput`,
  and the content of `acp.StartEditToolCall` take a `json.RawMessage`.
- Utility: `acp.Ptr[T]` for pointer fields in request/update structs.
- Reading values: `SessionInfo.Updated()` parses `updatedAt`;
  `RequestPermissionResponse.Chosen()` gives the selected option;
  `acp.MetaObject` and `acp.DecodeMeta[T]` read a namespaced `_meta` entry;
  `acp.ConfigOptionByCategory` finds a select option by category, and
  `SessionConfigSelectOptions.All()` lists its values, grouped or not.
- Every other union, except `AgentResponse` and `ClientResponse`, which the
  schema keeps out of its documentation, has a generated constructor for each
  of its variants, `acp.New<Union><Variant>`, such as
  `acp.NewSessionConfigOptionBoolean(id, name, currentValue)`. Its parameters
  are the variant's required keys, in the order of the schema's `required`
  list, each named after its key: a key that is a Go keyword or
  predeclared name gains `Value` (`typeValue`, `errorValue`), and a key that
  is not a Go name is joined in camel case (`x-API-key` gives `xAPIKey`). A
  variant whose discriminator the constructor cannot write itself takes it
  as a parameter: a catch-all, such as
  `acp.NewElicitationPropertySchemaOther(typeValue)`, keeps any other keys in
  its `Extra` field. A variant that is not an object takes `value`, such as
  `acp.NewRequestIdNumber(value int64)`.

### Wire encoding

The SDK encodes and decodes messages with `encoding/json` (v1). The generated types
follow the schema:

- Encoding a message whose required union has no variant set fails with an error
  that names the union, such as `RequestPermissionOutcome: no variant set`.
- A required array or map left nil is sent as `[]` or `{}`, not `null`, and so
  is a union's array variant that is set and holds nil, such as
  `acp.NewSessionConfigSelectOptionsUngrouped(nil)`.
- A key is read as `encoding/json` reads a struct field's name: a key that
  differs from the schema's name only in case, under Unicode case folding, is
  that name, and of several such keys the last wins. That holds for required
  keys, discriminators, and the keys that pick a variant or member, so
  `{"StopReason":"end_turn"}` decodes as `IdleStateUpdateEndTurn`; a catch-all's
  `Extra` keeps only the keys no field takes. Keys are sent under the schema's
  names.
- Decoding a union whose discriminator is present but not a string fails with an
  error that names the union and the key.
- A discriminator value this SDK does not know is not an error. Where the schema
  gives the union a catch-all variant, the payload decodes into it (`Other`),
  which keeps the keys it does not declare in its `Extra` field. Otherwise the
  payload is kept as received in the union's `Unknown` field. Either way,
  encoding sends the payload back unchanged.
- A payload without its discriminator, or with it `null`, decodes into the
  variant the schema means for that case where the union has one, such as
  `IdleStateUpdateNone`. Otherwise it decodes into the one variant whose own
  required keys it carries, not counting those the union requires of every
  variant; a payload that lacks one of those fails, naming the key. When none
  or several fit, it is kept in `Unknown`; a union with a catch-all instead
  fails with an error such as `StateUpdate: missing discriminator "state"`.
- Decoding JSON `null` into a union leaves it unchanged, except `RequestId`, which
  takes it as its `Null` variant. Any other payload replaces what the union
  held, so a reused value holds only the last one decoded.
- A number takes the Go type of its schema `format` (`uint64`, `uint32`,
  `uint16`, `int64`, `int32`, `float64`), so a value outside that range fails
  to decode, unless the property is lenient (below). A map whose schema types its values is a typed map, such as
  `map[string]string` for `env`.
- A value the schema gives no Go type, such as `rawInput`, `rawOutput`, or an
  error's `data`, is a `json.RawMessage`, carried byte for byte. Integers keep
  every digit there, through a union, and in sized integer fields; inside
  `_meta`, a `map[string]any`, numbers decode as `float64`. Invalid UTF-8 a peer
  sends in a raw value is carried too, where a string field replaces it with
  U+FFFD.
- An optional raw value that is absent or `null` is empty, as ACP defines
  omission and `null` to be equivalent for `rawInput` and `rawOutput`: test
  it with `len(v) == 0`, read it with `string(v)`, which gives `""`, and it is
  not sent. Formatted with `fmt` (`%v`, `%s`, `fmt.Sprint`), an empty
  `json.RawMessage` prints as `null` under Go 1.27's default build, where it is
  `jsontext.Value` ([golang/go#81050](https://github.com/golang/go/issues/81050)).
  Under Go 1.26's default build it prints as `[]` with `%v` and `fmt.Sprint`,
  and as an empty string with `%s`.
  A required raw value, such as a response's `result`, keeps `null`, and so
  does the unstable `UnstableMcpError`'s `data`, whose schema says "explicit
  null is retained": there `nil` is absent, `null` is sent as `null`, and
  `string(v) == "null"` tells the two apart.
- An object whose schema allows keys it does not declare
  (`additionalProperties: true`), such as `UnstableMcpError`, keeps them in
  its `Extra` field as received, and sends them back; a declared field wins
  over an `Extra` key of the same name.
- An elicitation form keeps the order of its fields. A decoded
  `ElicitationSchema` holds the keys of `properties` in `PropertyOrder`, in
  the order the agent wrote them, and sends them back in that order. An agent
  sets `PropertyOrder` to choose the order it sends: the keys it names come
  first, in its order, then any others, sorted; a name with no property is
  skipped. With no `PropertyOrder`, the keys are sent sorted, as `encoding/json`
  sends a map.
- Decoding a struct fails when a required key is absent, naming the type and
  the property, such as `PromptRequest: missing required property "sessionId"`;
  the connection answers such a request with `-32602`.
- A property the schema marks `x-deserialize-default-on-error` takes its schema
  default, or its zero value, when its value is malformed, and an array marked
  `x-deserialize-skip-invalid-items` drops the items that fail to decode, as
  ACP's Rust implementation does.
- `Validate` checks what the schema states, at every depth: a union has exactly
  one variant, or `Unknown`, set; a string meets its `minLength`; every nested
  value, slice element, and map value validates; a catch-all does not hold a
  discriminator value that names another variant, such as
  `ElicitationPropertySchema: variant Other: type "string" names another variant`.
  It also checks that each path the protocol requires to be absolute is, with
  the host's `filepath.IsAbs` (see Paths, below). Elsewhere a present empty
  string passes. Every failure is a `*acp.ValidationError` naming the type, the
  property and the rule.
- A decode error names the union and, for a variant, the discriminator value or
  the variant, then the cause, such as `ContentBlock: type "text": ...`.

#### Patch fields

Twelve properties of ACP's update messages have patch semantics: left out, the
receiver keeps what it stored; `null`, it clears it; a value, it replaces it.
Each is an `acp.Patch[T]`, which holds those three states:

| Types | Fields |
| :--- | :--- |
| `CompactionUpdate`, `SessionCompactionUpdate` | `Summary` `Patch[[]ContentBlock]`, `Error` `Patch[string]`, `Meta` `Patch[map[string]any]` |
| `SessionInfoUpdate`, `SessionSessionInfoUpdate` | `Title` and `UpdatedAt` `Patch[string]` |
| `SubagentUpdate`, `SessionSubagentUpdate` (unstable) | `Title` and `Description` `Patch[string]`, `Capabilities` `Patch[SubagentSessionCapabilities]`, `State` `Patch[StateUpdate]`, `Meta` `Patch[map[string]any]` |
| `SessionMessage`, `SessionUpdateSessionMessage` (unstable) | `Content` `Patch[[]ContentBlock]`, `Meta` `Patch[map[string]any]` |

The zero value is absent, and a field left unset is not sent.
`acp.PatchValue(v)` replaces the stored value and `acp.PatchNull[T]()` clears
it, sent as `null`:

```go
update := acp.SessionInfoUpdate{
    Title:     acp.PatchValue("Refactor the parser"),
    UpdatedAt: acp.PatchNull[string](),
}
```

A receiver tells the three states apart with `Value`, `IsNull` and `IsZero`:

```go
if v, ok := u.Title.Value(); ok {
    title = v
} else if u.Title.IsNull() {
    title = ""
} // absent: keep the stored title
```

- A value is sent as its type encodes: `acp.PatchValue([]acp.ContentBlock{})`
  is sent as `[]`, and a value holding a nil slice or map as `null`. An empty
  value read back is a value.
- For `summary` and `content`, ACP also has `[]` clear. The SDK reads `[]` as a
  value, and the receiver applies that rule.
- A value that does not decode is read as absent, as the schema's
  `x-deserialize-default-on-error` has it for these properties; an array drops
  the items that do not decode.
- `Validate` checks the value a patch holds.

#### Paths

ACP requires every file path in the protocol to be absolute. The SDK checks the
17 properties that carry one: `cwd` and `additionalDirectories` of
`session/new`, `session/load`, `session/resume` and the unstable
`session/fork`; the `cwd` filter of `session/list` and the `cwd` and
`additionalDirectories` of each session it lists; the `command` of a stdio MCP
server; the `path` of `fs/read_text_file` and `fs/write_text_file`; the `cwd`
of `terminal/create`; and the `path` of a tool-call location and of a diff.
A failure is a `*acp.ValidationError` with `Rule` `acp.RuleAbsolutePath`, such
as `NewSessionRequest: cwd: "" is not an absolute path`.

What the SDK sends is checked with no way to turn it off. The typed methods,
`SendRequest`, `SendRequestNoResult` and `SendNotification` return the error
and write nothing; a handler's result that fails is not sent, and the peer gets
`-32603`. The sender cannot know the receiver's operating system, so a path
absolute in POSIX or Windows form passes (`/home/u/p`, `C:\p`, `C:/p`,
`\\server\share`), and one absolute in neither fails (`""`, `src`, `./src`,
`\src`, `C:src`). Extension methods with raw JSON params are not checked. A
client that forwards an MCP server configured by name resolves it first, for
example with `exec.LookPath`.

What the SDK receives, every request, notification and response, is checked
with the receiving host's `filepath.IsAbs`, so on Linux and macOS `C:\p` is
refused. In order:

1. the message is decoded; a request that does not decode is answered
   `-32602`, and nothing that fails to decode reaches a hook;
2. `acp.WithInboundRewrite` gets a pointer to the decoded value and may change
   it;
3. the SDK validates what the rewrite left;
4. `acp.WithInboundValidation` gets the SDK's error, nil when the message is
   valid, and returns the error to act on, or nil to accept;
5. with an error, a request is answered with it (a `*acp.RequestError` as it
   is, a method that what `initialize` negotiated does not allow as `-32601`,
   any other error as `-32602`), a notification is dropped and logged, and
   a response's error is returned to the caller, wrapping the
   `*acp.ValidationError` when the error is the SDK's. Each message refused
   at steps 2 to 4, or by what `initialize` negotiated, adds one to the
   connection's `RefusedMessages()`; one that never decodes does not.

A rewrite hook can resolve what a peer sends as a bare name, or translate a
path from another host, such as a Windows client's path for an agent in WSL.
`acp.ResolveStdioCommands(exec.LookPath)` is the first, for every request that
carries MCP servers; a hook of your own looks like this:

```go
asc := acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin,
    acp.WithInboundRewrite(func(ctx context.Context, method string, params any) error {
        req, ok := params.(*acp.NewSessionRequest)
        if !ok {
            return nil
        }
        for i := range req.McpServers {
            if s := req.McpServers[i].Stdio; s != nil && !filepath.IsAbs(s.Command) {
                path, err := exec.LookPath(s.Command)
                if err != nil {
                    return err
                }
                s.Command = path
            }
        }
        return nil
    }))
```

A validation hook can accept a violation the application chooses to handle, or
add a rule of its own. Accepting one accepts a message the standard forbids,
which the application then answers for; `Validate` stops at the first failure,
so the rest of that message is not checked:

```go
acp.WithInboundValidation(func(ctx context.Context, method string, params any, err error) error {
    var ve *acp.ValidationError
    if errors.As(err, &ve) && ve.Type == "McpServerStdio" && ve.Field == "command" {
        return nil // this agent resolves a bare command itself
    }
    return err
})
```

A handler sees what the rewrite hook left, not what the peer sent. Absolute is
not confined: `/a/../../etc` is absolute. `Roots()` on `NewSessionRequest`,
`LoadSessionRequest`, `ResumeSessionRequest`, `UnstableForkSessionRequest` and
`SessionInfo` returns the session's root set, `cwd` and then
`additionalDirectories`; an agent that confines file access opens each root with
`os.OpenRoot` and works through the `*os.Root`, which refuses a path that leaves
it, symbolic links included.

The decisions behind these rules are
[0009-MADR-fix-generated-output-on-json-v1.md](docs/decisions/0009-MADR-fix-generated-output-on-json-v1.md),
[0011-MADR-harden-code-generator-engine.md](docs/decisions/0011-MADR-harden-code-generator-engine.md),
[0012-MADR-schema-fidelity-framework.md](docs/decisions/0012-MADR-schema-fidelity-framework.md),
[0014-MADR-union-constructors-take-every-required-key.md](docs/decisions/0014-MADR-union-constructors-take-every-required-key.md),
[0015-MADR-validate-absolute-protocol-paths.md](docs/decisions/0015-MADR-validate-absolute-protocol-paths.md),
and
[0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](docs/decisions/0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md).

### Initialization and capabilities

A connection starts with `initialize`. `ClientSideConnection.Initialize` records
the request and the agent's answer as an `acp.Negotiation` when the answer
arrives, before the connection reads what the agent wrote after it, and returns
the answer it recorded; `AgentSideConnection` records the same before its
answer is written. So a message either side sends the moment it has the answer
is judged by the record. A caller whose context ends, or whose connection
closes, once the answer has arrived still gets the answer, and the record holds
it. `Negotiated()` on either side returns the record and whether there is
one; a later `initialize` replaces it. The record is the JSON form of the
request and the answer as they went over the wire: it shares no memory with the
values passed to or returned by `Initialize` or a handler, and each call of
`Negotiated()` returns a new copy, so changing one changes nothing the SDK
checks.

The protocol version is `acp.ProtocolVersionNumber`. Left zero, in the request
or in an agent's answer, it is filled in. A client asked to send another gets a
`*acp.ProtocolVersionError` and nothing is sent; an agent whose handler answers
another is answered `-32603`. An agent that answers another version makes
`Initialize` return a `*acp.ProtocolVersionError` with `Requested` and
`Answered`, and records nothing, so every typed method keeps refusing. The
specification leaves the rest to the application: close the connection and
tell the user.

Until the record exists, every typed method but `Initialize` returns an error
wrapping `acp.ErrNotInitialized` and writes nothing, and a request received is
answered `-32601`. On the client, while `Initialize` waits for its answer, a
message the agent sends that only a record could allow is held instead: once
the answer is recorded it is judged against the record, and it is refused, as
before, when `initialize` fails by an error answer, another protocol version,
the caller's context ending or the connection closing. Before `Initialize` is
called nothing is held. A held notification waits in the notification queue,
with the notifications behind it, and a held request keeps its request slot,
so an agent that sends more of them before its answer than
`acp.WithMaxQueuedNotifications` or `acp.WithMaxConcurrentRequests` allows
meets those limits: the overflow policy, or `-32603`. Under
`acp.OverflowBlock`, the default, the connection reads on while `Initialize`
waits, to twice the queue's size, so held notifications do not stop it; under
`acp.OverflowCloseConnection` they close it. After the record, the SDK checks what each side sends and
receives against the capabilities in the record:

- Methods: `session/load` needs `loadSession`; `session/list`, `session/delete`,
  `session/resume`, `session/close` and the unstable `session/fork` need the
  matching `sessionCapabilities` entry; `logout` needs `auth.logout`;
  `fs/read_text_file` and `fs/write_text_file` need `fs.readTextFile` and
  `fs.writeTextFile`; the `terminal/*` methods need `terminal`; and
  `elicitation/complete` needs `elicitation.url`.
- Values: a non-empty `additionalDirectories` on `session/new`, `session/load`,
  `session/resume` and `session/fork` needs
  `sessionCapabilities.additionalDirectories`; an `http`, `sse` or unstable
  `acp` MCP server, on any of those four methods, needs `mcpCapabilities.http`,
  `.sse` or `.acp`; an `image`,
  `audio` or `resource` block in a prompt needs `promptCapabilities.image`,
  `.audio` or `.embeddedContext`; an elicitation's `form` or `url` mode needs
  `elicitation.form` or `.url`, and an elicitation in any other mode, which no
  capability advertises, is refused, as `elicitation.<mode>`, with every
  capability advertised; a `notice` update needs `session.notices`, a
  compaction update `session.compaction`, and the unstable `subagent_update`,
  `session_message` and `session_message_chunk` need `subagents`; a `boolean`
  config option, wherever an agent sends one, needs
  `session.configOptions.boolean`; and a `terminal` auth method in the
  `initialize` answer needs `auth.terminal`.

A typed method that would send what the record does not allow returns a
`*acp.CapabilityError` naming the method and the capability, and the property
(`prompt[1]`) or the variant (`image`) for a value; nothing is written. A
handler's result that breaks a value rule is not sent, and the peer gets
`-32603`. What is received goes through the receiving order of
[Paths](#paths): a request the record does not allow is answered `-32601` for a
method and `-32602` for a value, a notification is dropped and logged, and a
response's error is returned to the caller. The answer to `initialize` is
checked against the record it is about to make, so an agent offering a terminal
auth method to a client without `auth.terminal` answers `-32603`, and a client
given one returns the error and records nothing.

The client's inbound hooks see the `initialize` answer once, on the goroutine
that reads the connection, before anything after it is read: what a rewrite
hook makes of it is what `Initialize` returns and what is recorded, an answer
the validation hook refuses records nothing, and a hook that blocks on it holds
every message behind it.

`Authenticate` with the id of a `terminal` auth method the agent listed returns
an error wrapping `acp.ErrTerminalAuthMethod` and sends nothing, since a
terminal method is run, not authenticated; received, it is answered `-32602`.

`Negotiation.Allows(method)` reports whether the record lets a method be sent,
for an application that shows or hides features: nil, or the
`*acp.CapabilityError`. `Negotiation.AllowsValue(method, params)` runs every
check a send runs, the method's and those of its values, such as an MCP
server's transport or a prompt's image block, and returns the error the send
would, without sending, so a caller can choose another value first.

A validation hook sees these errors after the SDK's own validation: a message
that fails validation reaches the hook with that error first and, only if the
hook accepts it, with the capability error, so accepting one never accepts the
other. A hook can accept a peer that does not follow the record:

```go
acp.WithInboundValidation(func(ctx context.Context, method string, params any, err error) error {
    var ce *acp.CapabilityError
    if errors.As(err, &ce) && ce.Capability == "fs.readTextFile" {
        return nil // this client serves reads it did not advertise
    }
    return err
})
```

`Connection()`, `SendRequest`, `SendRequestNoResult`, `SendNotification`, and
`CallExtension` and `NotifyExtension` hold no record: they send any method,
before `initialize` too. What is received holds to it, extension messages
included (see [Extension methods](#extension-methods)). The unstable features whose capabilities the
specification does not tie to a rule (`nes`, `plan`, `providers`,
`positionEncoding`) are not checked.

### Cancellation

An agent's `Prompt` handler runs under a context that `session/cancel` for its
session cancels, with `acp.ErrPromptCancelled` as `context.Cause`; by
default, a second prompt for the same session cancels the first with an error
that wraps it (see the prompt-overlap policy below).
When the handler returns, if its context was cancelled, by `session/cancel` or
`$/cancel_request`, the answer is `stopReason` `cancelled`, keeping the rest of
the handler's result, whatever the handler returned. A turn whose connection
has ended is not answered: nothing is written after the end. A `session/cancel` that
arrives after the handler has returned changes nothing. A prompt and a cancel
are ordered as they were received, so a cancel sent right after a prompt
cancels that turn. A cancel applies to every prompt of its session received
before it, two prompts in flight included. Any other request whose handler
`$/cancel_request` cancels, and that returns the context's error, is answered
`-32800`.

`acp.WithPromptOverlapPolicy` chooses what an `AgentSideConnection` does with a
prompt for a session whose turn is still running, which ACP leaves to the
agent:

- `acp.PromptOverlapSupersede`, the default: the running turn is cancelled,
  with a cause that wraps `acp.ErrPromptCancelled`, and the new prompt runs. A
  prompt received before the running turn's is cancelled at once instead.
- `acp.PromptOverlapRefuse`: the new prompt is answered at once, `-32603` with
  `data` `{"error": "a prompt turn is already running for this session",
  "sessionId": <id>}`, and `Prompt` is not called; the running turn goes on. A
  turn that `session/cancel` or `session/close` has cancelled no longer counts,
  so a prompt sent after a cancel runs, even before the cancelled handler
  returns.
- `acp.PromptOverlapConcurrent`: every prompt calls `Prompt`, and the agent
  queues, steers or refuses in its own handler. `session/cancel` and
  `session/close` cancel every running turn of the session received before
  them.

A value the package does not define is the default, and the other connections
ignore the option. Under every policy, `$/cancel_request` cancels only its own
request.

```go
asc := acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin,
    acp.WithPromptOverlapPolicy(acp.PromptOverlapRefuse))
```

`session/close` ends its session's work as `session/cancel` does: before the
agent's `CloseSession` handler runs, each running turn of the session is
cancelled and answered `cancelled`. When the agent answers a client's `CloseSession` with
success, each `session/request_permission` of that session still pending is
answered `cancelled`, as on `Cancel`; a close the agent refuses leaves them to
their handlers.

Notifications are handled one at a time, in the order received, and a response
is returned to its caller only after the notifications received before it are
handled. A notification handler, such as a client's `SessionUpdate`, that sends
a request on its own connection must send it under the handler's context, or
one derived from it: such a request does not wait for the handler that sent
it. Sent under an unrelated context, such as `context.Background()`, it waits
for that handler, which is waiting for it.

`ClientSideConnection.Prompt` sends `session/cancel` when its context ends with
an error, and returns. When a client sends `session/cancel`, through `Cancel` or
`Prompt`, each `session/request_permission` of that session still pending is
answered `cancelled` at once; its handler's context is cancelled with
`acp.ErrPromptCancelled`, and what it returns later is discarded. A permission
request that arrives after the cancel is handled as usual. Marking the turn's
unfinished tool calls `cancelled`, which the specification recommends, is the
client application's.

### Closing and limits

`Close()`, on a connection or on either side connection, ends it: `Done()`
closes, every call waiting for an answer returns, every later send fails
without writing, every handler's context ends, a request's and a
notification's alike, and the reader and the writer are closed when they
implement `io.Closer`, which also ends a read in progress. A second `Close`
returns nil. When the peer ends the connection, the same holds, but for the
transport, which is the application's to close. A full notification queue
pauses the connection by default, and closes it only under the overflow
policies below that say so.

`Connection.Shutdown(ctx)` is the graceful close, as `http.Server.Shutdown` is
a server's. Every handler's context ends with `acp.ErrShutdown`, and the
connection keeps writing until each request it received before is answered:
an agent's prompt whose handler honours its context is answered `cancelled`,
as after `session/cancel`. Then it closes, and `Err()` wraps `acp.ErrShutdown`
beside `acp.ErrConnectionClosed`. A request received meanwhile is answered
`-32603`, and the connection keeps reading, so the answers to this side's own
requests still arrive. When ctx ends first, it closes at once and returns
ctx's cause. A side connection's is its `Connection()`'s.

`acp.ServeAgent(ctx, agent, in, out, opts...)` is an agent's main. It serves
until the input ends, waits up to 5 seconds for the handlers, and returns nil
at a clean end; when ctx ends, as at an interrupt, it shuts the connection
down within the same 5 seconds and returns ctx's cause (`ExampleServeAgent`;
[0042-MADR-in-process-agent-runner.md](docs/decisions/0042-MADR-in-process-agent-runner.md),
item 2). When the 5 seconds pass with handlers still running, at either end,
its error wraps `acp.ErrHandlersRunning`, which `errors.Is` reads (0042, the
amendment of 2026-10-30).

The SDK closes no connection on a timer: an idle connection is normal in ACP,
and the protocol has no ping to tell an idle peer from a dead one. The
application decides how long a connection lives. A request's deadline is its
context, a session ends with `session/close`, and an agent process is the
client's to end, which the `agentproc` package does
([0023-MADR-connection-lifetime-and-test-leaks.md](docs/decisions/0023-MADR-connection-lifetime-and-test-leaks.md)).

`agentproc` starts an agent as a child process and stops it with the processes
it started, every wait for the agent bounded
([0027-MADR-agent-process-helper.md](docs/decisions/0027-MADR-agent-process-helper.md));
the wait for the client's own handlers is bounded by `CloseContext` and
`KillContext`, below.
`Command.Start` connects a `ClientSideConnection` over the agent's standard
input and output, through pipes it owns, so nothing the agent writes before it
exits is lost; the command's `Stderr`, `Env` and `Dir` stay the caller's. On
Unix the agent runs in a process group of its own, and on Windows in a job
object; on Windows the agent is started suspended and runs only once it is in
the job, so a process it starts at once is in the job too, and `Start` refuses
a command whose `CreationFlags` has `CREATE_SUSPENDED` already. A process the
agent's tree starts in a session of its own on Unix, or with
`CREATE_BREAKAWAY_FROM_JOB` on Windows, is outside the group or the job: what
ends them does not end it, and it is the agent's to end. Should the client die
without `Close`, the agent's input ends, which
ends an agent that exits at its input's end; beyond that, on Windows the job
ends the agent and the processes it started, and on Linux and FreeBSD the
kernel sends the agent SIGKILL, unless the caller set its own `Pdeathsig`,
while a process it started ends only at its input's end; elsewhere nothing
more ends them
([0029-MADR-unix-client-death-pdeathsig.md](docs/decisions/0029-MADR-unix-client-death-pdeathsig.md)).
`Close`
closes the agent's input and waits up to the grace, 5 seconds unless
`Command.Grace` says otherwise; on Unix it then sends SIGTERM to the group and
waits again; then it kills the group, or ends the job. Once the agent has
exited, the connection reads to the end of the agent's output, up to the grace,
and is closed. `Close` and `Wait` return the agent's exit error, joined with
`agentproc.ErrTerminated` or `agentproc.ErrKilled` when `Close` had to
escalate. `Kill` runs the same steps without their waits, ending the agent and
the processes it started at once, with `agentproc.ErrKilled`, and an agent that
has exited is sent nothing; a `Close` in progress stops waiting and kills. When
the agent exits, what is left of its group or job is killed; when the
connection ends first, the agent is stopped. The package needs
`golang.org/x/sys`, on Windows only:

<!-- example: agentproc ExampleCommand -->
```go
ctx := context.Background()
cmd := exec.Command("my-agent", "--acp")
// The agent's log.
cmd.Stderr = os.Stderr
p, err := (&agentproc.Command{Cmd: cmd}).Start(client{})
if err != nil {
    log.Fatal(err)
}
defer func() {
    if err := p.Close(); err != nil {
        log.Printf("agent: %v", err)
    }
}()
if _, err := p.Conn().Initialize(ctx, acp.InitializeRequest{}); err != nil {
    log.Printf("initialize: %v", err)
}
```
<!-- /example -->

`Command.OutputTap` and `Command.InputTap` are writers that receive a copy of
every byte the agent writes to its output, and of what the connection writes to
its input, in order, before anything is decoded: a line too long or not JSON
included. Each tap is written on a goroutine of its own, so the connection
never waits for it, and its errors are ignored. A tap that falls 32 MiB behind
is detached: what it received is the stream's beginning, and
`Process.DroppedTapBytes` counts the rest. `Wait` gives the taps up to the
grace to write what they hold. `ExampleCommand_outputTap` records the agent's
output to a file. `agentproc.LineTap(fn)` is a tap that calls `fn` with each
line, a copy without its newline, as the connection reads or writes it, and
holds a line not yet ended.

`Process.ExitedFirst()` reports, once `Wait` has returned, whether the agent
exited while its connection was still open, rather than after the connection
ended; when the agent's own exit closes its output, the two can come together.
`Process.WaitContext(ctx)` is `Wait` bounded by a context: it returns
`context.Cause(ctx)` when ctx ends first, and leaves the agent running.

`Close`, `Kill` and `Wait` also wait for the connection's handlers, the
client's own methods, without bound: a handler that ignores its context holds
them until it returns, as it holds `Connection.Wait`. `Process.CloseContext(ctx)`
and `Process.KillContext(ctx)` are `Close` and `Kill` bounded by a context.
Should ctx end during `Close`'s steps, the steps left kill at once, as `Kill`'s
do; should it end before `Wait` would return, they return ctx's cause, joined
with `acp.ErrHandlersRunning` when a handler is what still runs, and with the
agent's error. The agent has ended by then, and `Wait` returns the whole error
once the handler has (`ExampleProcess_CloseContext`;
[0042-MADR-in-process-agent-runner.md](docs/decisions/0042-MADR-in-process-agent-runner.md),
the amendment of 2026-10-30).

`agentproc.InProcess{Serve: …}.Start(client)` runs an agent written in Go in
this process. `Serve(ctx, in, out)` is what the agent's own main runs, such as
`acp.ServeAgent`, here on a goroutine joined to the client by two `io.Pipe`s,
and the `*Process` is the one a child gets. When `Serve` returns, its output
and input are closed, as at a child's exit. `Close`'s steps are mapped onto a
goroutine: the first closes `Serve`'s input; the second cancels its context
with `agentproc.ErrTerminated`; the third cancels it with `agentproc.ErrKilled`
and closes its pipes. A `Serve` that still runs a grace later is abandoned, and
`Wait`'s error holds `agentproc.ErrAbandoned`, since Go cannot stop a
goroutine. A `Serve` that returns its context's cause, as `acp.ServeAgent`
does, already says how it was stopped, and `Wait` returns its error as it is,
rather than join the sentinel a second time. A panic in `Serve` is not recovered: an agent that must not take
its client down with it runs as a child (`ExampleInProcess`; 0042, item 1).

`Command.StderrTail` and `Command.StderrTailLines` keep the end of the agent's
standard error, at most so many bytes and lines, from a line's beginning, while
`Cmd.Stderr` still receives every byte. `Process.StderrTail()` returns it once
`Wait` has returned, and `Command.JoinStderrTail` joins a
`*agentproc.StderrError` holding it to a `Wait` error that is not nil (0042,
item 3).

`agentproc.NewTranscript(w)`, its `Input()` and `Output()` set as the two taps,
records both ways of the wire, one `{"direction","frame"}` object a line, the
form `acptest.ReadRecords` reads (`ExampleTranscript`; 0042, item 4).

When the input ends cleanly, at end of file, as when a script closes a stdio
agent's standard input after writing its requests, the requests already received
are answered first. The requests this side is waiting on fail at once, as do any
it sends later, `-32603` wrapping `acp.ErrPeerDisconnected`, since no answer can
arrive. The handlers already running keep their contexts and their answers are
written, for at most five seconds, after which their contexts are cancelled and
`Done()` closes. A read that fails otherwise ends the connection at once.

`Err()` says why a connection ended. It is nil until `Done()` closes, and then
the same error each time: `acp.ErrConnectionClosed` when this side ended it, by
`Close` or by a notification overflow, under `acp.OverflowCloseConnection` or
past `acp.OverflowBlock`'s read-on limit, which wraps
`acp.ErrNotificationQueueOverflow`;
otherwise `acp.ErrPeerDisconnected`, wrapping `io.EOF` when the peer ended its
output, or the read's error. The first end is the one kept: a `Close` after the
peer's end leaves `Err()` as it was, though a notification sent then fails with
`acp.ErrConnectionClosed`; a request still fails with `acp.ErrPeerDisconnected`,
as above. `Wait()` blocks until the connection has ended and
every goroutine it started has returned, its handlers' included, and then returns
`Err()`. It does not end the connection, so close it first. A reader that does
not implement `io.Closer` keeps `Wait()` waiting until its `Read` returns, and so
does a handler that ignores its context. Either side connection has both
([0026-MADR-connection-end-cause-and-wait.md](docs/decisions/0026-MADR-connection-end-cause-and-wait.md)).
`acp.EndedCleanly(err)` is true for an `Err()` of this side's `Close` or of the
peer's end of output, and false for an overflow's close, a read's error and
nil. `Connection.WaitContext(ctx)` is `Wait` bounded by a context; a side
connection's is its `Connection()`'s, since its own methods that take a
context are protocol methods:

```go
select {
case <-ctx.Done():
case <-asc.Done():
}
_ = asc.Close() // Err keeps an earlier end
switch err := asc.Wait(); {
case errors.Is(err, acp.ErrNotificationQueueOverflow):
    log.Print("the client's notifications overflowed the queue's limit")
case errors.Is(err, io.EOF):
    log.Print("the client closed its output")
}
```

Every send returns when its context ends, even while the peer does not read.
A message whose write has begun may still reach the peer.

`acp.WithMaxQueuedNotifications(n)` sets how many received notifications may
wait while a notification handler runs, 1024 by default.
`acp.WithNotificationOverflowPolicy(p)` sets what happens past that
([0044-MADR-notification-queue-back-pressure.md](docs/decisions/0044-MADR-notification-queue-back-pressure.md)):

- `acp.OverflowBlock`, the default since v1.29.0: the connection stops reading
  the peer until a handler returns, so the peer's writes wait. Nothing is
  dropped, and the connection stays open. An agent whose client is slow is
  slowed at its `SessionUpdate` calls, and `session/cancel` still ends its turn.
- `acp.OverflowCloseConnection`, the default before v1.29.0: the connection
  closes as `Close` does, and `Err()` wraps `acp.ErrNotificationQueueOverflow`.
- `acp.OverflowDropNewest`: the notification is dropped and counted by
  `DroppedNotifications()`, and the connection stays open.

While the reader is paused, nothing else is read either. A request from the
peer, such as a permission request, waits behind the queue, and so does a
`$/cancel_request`. A request this side sent still returns no later than it
would otherwise, since it returns only once the notifications received before
its answer are handled. So, under `acp.OverflowBlock`, a notification handler
must not wait for a request or a `$/cancel_request` the peer sends later.

The connection reads on, to twice the queue's size, while a notification
handler waits for the answer to a request it sent under its context, or for
`initialize`'s answer, since only the reader can deliver either. Past that, it
closes with `acp.ErrNotificationQueueOverflow`. A pause that lasts 5 seconds
is logged at `Warn`.

Over package `remote`, the server's own bounds still apply to a long pause: a
WebSocket write waits at most `Grace`, and Streamable HTTP holds at most
`MaxBuffered`. To keep the close that was the default before v1.29.0:

```go
conn := acp.NewClientSideConnection(client, stdin, stdout,
    acp.WithNotificationOverflowPolicy(acp.OverflowCloseConnection))
```

`acp.WithMaxConcurrentRequests(n)` bounds the received requests whose handlers
run at once. Over the limit, a request is answered at once, `-32603` with
`{"error": "too many concurrent requests", "limit": n}`, and its handler does not
run; the reader never waits for a handler, since a running one may be waiting
for a response only the reader delivers. Notifications and `$/cancel_request`
do not count. The default is no limit.

### Remote agents

Package `remote` runs ACP over the network. It implements the RFD
["Streamable HTTP & WebSocket Transport"](https://agentclientprotocol.com/rfds/streamable-http-websocket-transport)
at its revision of 2026-07-02, on both sides and in both of its profiles,
which share one endpoint:

- **Streamable HTTP:** a POST for each message to the agent, and server-sent
  event streams, one for the connection and one for each session, for the
  messages from it.
- **WebSocket:** one socket that carries every message both ways, one text
  frame each.

Both sides present the remote connection as the byte stream the rest of the
module reads and writes, one JSON-RPC message a line, as stdio carries it. So
connections, `acp.ServeAgent`, the taps and transcripts work over the network
as they do over a child's pipes
([0043-MADR-remote-http-and-websocket-transport.md](docs/decisions/0043-MADR-remote-http-and-websocket-transport.md)).

A client calls `remote.Dial(ctx, url, opts)`.

- **The URL:** `ws://` and `wss://` dial WebSocket, and `ctx` bounds the
  handshake. `http://` and `https://` dial Streamable HTTP, which sends nothing
  until the first message: it must be `initialize`. `ctx` never bounds the
  connection itself.
- **Joining it:** the `*remote.Conn` it returns is both of a connection's
  streams: `acp.NewClientSideConnection(client, c, c)`.
- **`DialOptions`:**
  - `HTTPClient` makes the requests; nil is a new client. A nil `Jar` becomes a
    cookie jar of the connection's own, since the RFD requires a client to
    keep the server's cookies. For WebSocket, an `*http.Transport` is copied
    and limited to HTTP/1.1, which the handshake needs. Leave its `Timeout`
    zero: it bounds reading a whole response body, so it would end the event
    streams.
  - `Header` goes on every request, the handshake included. Authentication
    goes there.
  - `MaxMessageSize` defaults to 10 MiB.
- **`Close`:** a WebSocket's close handshake, or Streamable HTTP's `DELETE`,
  waiting up to 5 seconds; `Read` then returns `io.EOF`.

Over Streamable HTTP, the client follows the three official servers:

- **Order:** messages are POSTed one at a time, in order, each waiting for its
  `202`.
- **`Acp-Session-Id`:** a request or notification for a session carries the
  header, unless its method is a `$/` one, and so does an answer to the
  agent's request that named a session. A POST answered `404` with it is sent
  once more without it, as Python's server wants.
- **Session streams:** a session's stream is opened before a `session/new`
  answer naming it is delivered, and before a `session/load` or
  `session/resume` is sent.
- **No order between streams:** the streams are read together, and the RFD
  orders nothing between them. A `session/load` answer can arrive before the
  replay its session's stream still carries, so wait for the updates you
  expect, as `example/remote` does.
- **Ends:** the connection stream's end ends the `Conn`, cleanly when the
  server ended it cleanly.

An agent is served by `remote.NewHandler(serve, opts)`, an `http.Handler` for
the endpoint.

- **Per connection:** it runs `serve(ctx, in, out)` once, on a goroutine of its
  own. `serve` is what an agent's main runs, usually `acp.ServeAgent`.
- **Methods:** a GET with `Upgrade: websocket` opens a WebSocket. A POST sends a
  message, and `initialize` opens a Streamable HTTP connection. A GET opens one
  of its event streams, and a DELETE ends it. Any other method, a browser's
  CORS preflight included, is `405`.
- **Answers to a POST:** `202` once its message is queued for `serve`, before
  `serve` reads it, so the `202` is sent before the agent can answer. The two
  travel on different connections, so a client may still read the answer
  first. While 256 messages wait, a POST waits for room.
- **`initialize`:** answered with the agent's own answer and the connection's
  id. An answer with an error is sent without an id, and the connection ends.
  A `serve` that returns first gets `500` with `-32603`, and no answer within
  30 seconds gets `504`.
- **The session rule:** a message's session comes from `Acp-Session-Id` or from
  `params.sessionId`, as Rust's server takes it. So the TypeScript, Python
  and Rust clients are all served.
- **`HandlerOptions`:**
  - `Grace`, 5 seconds by default, bounds each step of a connection's ending
    and each WebSocket message written to a client.
  - `KeepAlive`, 15 seconds, is how often each WebSocket is pinged and each
    event stream sent a comment, traffic or none. A ping not answered within
    it ends the connection.
  - `MaxMessageSize` is 10 MiB.
  - `MaxBuffered`, 64 MiB, bounds what a Streamable HTTP connection holds for
    streams no client is receiving.
  - `OriginPatterns` allows browser origins besides the request's own host,
    on every method, as the WebSocket upgrade matches them. A request with no
    `Origin`, as a program sends, is always allowed.
  - `Logger` gets byte counts and fixed labels, never a message, an id or a
    session id.
- **Event streams:** the handler clears their write deadline, so the server's
  `WriteTimeout` does not cut them.

A connection ends through one ladder, as `agentproc.InProcess` does:

1. close `serve`'s input, and wait up to `Grace`;
2. cancel its context with the cause, and wait again;
3. close its pipes, and wait a last time.

A `serve` still running is then abandoned. Its context's cause, which
`context.Cause` reports, says why the connection ended:

| Cause | When |
| :--- | :--- |
| `remote.ErrDeleted` | the client sent `DELETE` |
| `remote.ErrDisconnected` | the client closed its socket, stopped answering pings, or left its Streamable HTTP connection with no connection stream for 60 seconds |
| `remote.ErrHandlerClosed` | the handler was closed |
| `remote.ErrOverflow` | the messages held past `MaxBuffered` |

`Handler.Close(ctx)` ends every connection at once and answers each later
request `503`. It returns `ctx`'s cause when `ctx` ends first, and an error
wrapping `remote.ErrAbandoned` when a `serve` had to be abandoned. A WebSocket's close handshake waits up to 5 seconds for a
client that does not answer it.

The host's part is the rest, as the RFD layers it on the transport:

- **Authentication**, as middleware before the handler, on every request. The
  RFD says `Acp-Connection-Id` is not a credential, so checking `initialize`
  alone is not enough.
- **Limits** on how many connections one client may open.
- **CORS** for browsers, the preflight included.
- **TLS.**
- **Shutdown order:** `Handler.Close` before `http.Server.Shutdown`, which waits
  for every open connection, an event stream included.

<!-- example: remote ExampleNewHandler -->
```go
h := remote.NewHandler(func(ctx context.Context, in io.Reader, out io.Writer) error {
    return acp.ServeAgent(ctx, agent{}, in, out)
}, nil)
mux := http.NewServeMux()
// Authenticate every request here, not only initialize: the connection's
// id is not a credential.
mux.Handle("/acp", h)
srv := &http.Server{Addr: "127.0.0.1:7331", Handler: mux, ReadHeaderTimeout: 10 * time.Second}
go func() {
    if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
        log.Fatal(err)
    }
}()
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
defer stop()
<-ctx.Done()
// Close first: Shutdown waits for every open connection, and an open
// event stream is one until the handler ends it.
bound, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
if err := h.Close(bound); err != nil {
    log.Printf("handler: %v", err)
}
if err := srv.Shutdown(bound); err != nil {
    log.Printf("server: %v", err)
}
```
<!-- /example -->

`example/remote` is a program of both sides.

- `remote serve -addr ADDR` serves an echo agent that can load its sessions. It
  enables HTTP/1.1 and unencrypted HTTP/2, so a client that speaks HTTP/2 with
  prior knowledge over `http://` is served too.
- `remote dial URL` runs a prompt and, when the agent can, loads the session on
  a second connection.
- `tools/interop.py` runs both against the official TypeScript and Python
  SDKs, at pinned commits, over both profiles.

Not supported:

- resuming an event stream with `Last-Event-ID`, and reconnecting
  automatically. A client dials again and loads its session.
- batches, answered `501`;
- WebSocket subprotocols and compression, and WebSocket over HTTP/2;
- Unix sockets.

The package is outside the compatibility promise ([Stability](#stability)).

### Errors on the wire

The connection reads one JSON-RPC message per line and answers what JSON-RPC
2.0 says to answer:

- a line that is not JSON: `-32700`, with `"id": null`;
- JSON that is not an object, a batch included: `-32600`, with `"id": null`;
- an object that is not a valid request or notification (`jsonrpc` other than
  `"2.0"`, `method` absent or not a string, an `id` that is not a string,
  number or `null`, `params` that is present and is not an object, an array or
  `null`): `-32600`, with its
  id when the id is valid and `null` otherwise, an absent id included;
- a line longer than the limit: read through its newline, discarded, logged,
  and answered `-32600` with `"id": null` and the limit in `data`; reading goes
  on. The limit is 10 MiB unless `acp.WithMaxMessageSize(n)` sets another.

A request whose id is `null` is answered with `"id": null`. A response is never
answered: one that matches no pending call is logged at Debug, and one that
holds neither or both of `result` and `error`, or another `jsonrpc`, fails the
call it answers. `acp.NewResourceNotFound(uri)` builds the `-32002` error ACP
defines for a resource, such as a file, that does not exist.

`params` left out, or `null`, which ACP's schema allows, read as empty params:
a method whose params have no required property, such as `logout` or
`session/list`, is handled, and one with required properties is answered
`-32602` naming the one missing. The SDK never sends `params` as `null`: params
that encode as `null` are left out, and params that encode as neither an
object nor an array are refused `-32602` before anything is written. A request
whose context has already ended is not written.

Each method is handled as the kind the schema makes it: a request-only method
sent without an id runs no handler and is logged, and a notification-only
method sent with an id is answered `-32601`. When two requests in flight carry
the same id, `$/cancel_request` for it reaches the later one. A handler that
returns a nil `*acp.RequestError` as a non-nil `error` is answered `-32603`.

An error the connection makes for a local failure keeps that failure, for
`errors.Is`: a call whose context ended wraps `context.Cause(ctx)`, which is
`context.Canceled` or `context.DeadlineExceeded` when no cause was given; one
whose connection ended wraps
`acp.ErrPeerDisconnected`, or `acp.ErrConnectionClosed` after `Close`; one
whose write failed wraps the write's error. Its code is as before. An error the
peer answered wraps nothing, so a peer's `-32800` is not `context.Canceled`.

An error a handler returns reaches the peer. A `*acp.RequestError` is sent as
it is; any other error is answered `-32603`, or `-32800` when it wraps
`context.Canceled`, with its text in the answer's `data` as `"error"`. A
message that names a local path or holds a token reaches the peer as it is,
so return a `*acp.RequestError` of your own to keep it private. An agent's
`Prompt` and a client's `RequestPermission` are the exceptions: once their
prompt turn is cancelled, they are answered cancelled, whatever they return.

To classify an error a call returned, wrapped or not: `acp.IsMethodNotFound`
is true for a peer's `-32601` and for a method the peer did not advertise,
which this side refuses before sending; `acp.IsAuthRequired` for `-32000`;
`acp.IsResourceNotFound` for `-32002`, a file or terminal the client does not
have; and `acp.IsCancelled` for `context.Canceled`, `acp.ErrPromptCancelled`
and `-32800`. `(*acp.RequestError).Reason()` gives the `"error"` of its `data`,
where this SDK writes why it refused, or else its message.

A malformed message is logged by its fault, its size and, when it has a valid
one, its id, never by its content, which may hold prompt text or file contents.
A message that breaks the schema is refused with an `*acp.ValidationError`;
for an absolute path or an excluded value, its text quotes the value refused,
so that value reaches the answer's `data` and the logs as it was.

These rules are decided in
[0017-MADR-remediate-acp-v1-conformance-violations.md](docs/decisions/0017-MADR-remediate-acp-v1-conformance-violations.md)
and
[0019-MADR-remediate-debugging-pass-findings.md](docs/decisions/0019-MADR-remediate-debugging-pass-findings.md).

### Elicitation

An agent asks the user for input with `elicitation/create`, in form mode or URL
mode, and only in a mode the client advertised in `clientCapabilities.elicitation`,
`form` or `url`: `CreateElicitation` returns a `*acp.CapabilityError` for any
other, and a client answers one it receives `-32602`. Unlike MCP, an empty
`elicitation: {}` advertises no mode. The specification asks the client to check
a form's answer before sending it, and the agent to check it again; the helpers
on `acp.ElicitationSchema` do both the same way
([0040-MADR-check-elicitation-forms-answers-and-urls.md](docs/decisions/0040-MADR-check-elicitation-forms-answers-and-urls.md)).

A client shows the form and checks the user's answer:

```go
func (c *MyClient) CreateElicitation(ctx context.Context, req acp.CreateElicitationRequest) (acp.CreateElicitationResponse, error) {
    if req.Form == nil {
        return acp.NewCreateElicitationResponseCancel(), nil
    }
    schema := req.Form.RequestedSchema
    // Fields come in the order the agent wrote them; Defaults pre-fill them.
    answers, ok := c.ask(ctx, req.Form.Message, schema.Fields(), schema.Defaults(), func(content map[string]acp.ElicitationContentValue) error {
        return schema.Check(content) // shown to the user, who corrects the answer
    })
    if !ok {
        return acp.NewCreateElicitationResponseDecline(), nil
    }
    resp := acp.NewCreateElicitationResponseAccept()
    resp.Accept.Content = answers
    return resp, nil
}
```

- `Fields()` lists the properties in the order the form was sent, each with
  whether it is required. `Kind()` names a field's type; a type this package
  does not know is a field too, which a client must not render as a known
  control.
- `ParseValue(text...)` turns what a text interface collected into the value its
  field takes, without rounding or trimming: `3.7` is not an integer.
- `Check(content)` returns nil, or every problem at once, each a
  `*acp.ValidationError` whose `Field` is `content["<name>"]` and whose message
  never quotes the value. A required field must be present, though an empty
  string or list is present; a key the form does not declare is refused;
  lengths count characters; bounds compare exactly; `email` is an RFC 5321
  mailbox, `uri` an absolute URI, `date` and `date-time` RFC 3339; and a
  `pattern` is matched by Go's RE2, one RE2 cannot compile reported as
  `acp.RuleUnsupported`, never as a match.
- `CheckSchema()` reports a form no answer can satisfy, or that is ambiguous,
  such as a `required` name with no property; nothing runs it unasked.

An agent checks the answer it receives, and handles `decline` and `cancel`,
which a client may send at any time:

```go
resp, err := conn.CreateElicitation(ctx, req)
switch {
case err != nil:
    return err
case resp.Accept != nil:
    if err := req.Form.RequestedSchema.Check(resp.Accept.Content); err != nil {
        return err
    }
    age, _ := resp.Accept.Content["age"].Int64() // an Integer, or 5.0 as a Number
    _ = age
}
```

`acp.WithElicitationCheck()` makes the connection run `Check` itself, on the
side that sets it, on every `accept` of a form; a `decline`, a `cancel` and a
URL-mode `accept` pass as before. On a client, an answer that fails is not
sent: the agent is answered `-32603` with the problems. On an agent,
`CreateElicitation` returns an error wrapping them, and the zero response. A
refused answer cannot go back to the user who wrote it, so a client still calls
`Check` first, where the user can correct the answer, and keeps the option as
a backstop.

Form mode must never ask for a password, a token or any secret: the specification
requires URL mode for those, and no fallback to a form. An agent withdraws a
pending elicitation by ending the context of its `CreateElicitation` call, which
sends `$/cancel_request`.

In URL mode the agent sends the user to a page: `accept` means only that the
user agreed to open it. A client must not open or prefetch the URL without the
user's consent, and shows the whole URL and its host first. `acp.ParseElicitationURL`
refuses what no client should offer to open, a URL that is not absolute http or
https, has no host, carries userinfo, or holds whitespace, and gives the host to
show and whether to warn:

```go
u, err := acp.ParseElicitationURL(req.Url.Url)
if err != nil {
    return acp.NewCreateElicitationResponseDecline(), nil
}
if !c.consent(ctx, req.Url.Message, u.URL.String(), u.Host, !u.Secure || u.Punycode) {
    return acp.NewCreateElicitationResponseDecline(), nil
}
```

An agent gives each URL elicitation an id unique among those outstanding, such
as `acp.ElicitationId(rand.Text())` from `crypto/rand`, and may send
`elicitation/complete` with it when the user is done. A client ignores a
`CompleteElicitation` for an id it does not hold or has already completed; the
SDK keeps no record of them.

### Extension methods

ACP supports **extension methods** for custom JSON-RPC methods whose names start with `_`.
Use them to add functionality without conflicting with future ACP versions.

#### Handling inbound extension methods

Implement `acp.ExtensionMethodHandler` on your Agent or Client. Your handler will be
invoked for any incoming method starting with `_`.

Before it runs, the message goes through what a typed one does: both inbound
hooks, which see `params` as a `*json.RawMessage` that `acp.WithInboundRewrite`
may replace, and the record of `initialize`. Before `initialize`, an extension
request is answered `-32601` and an extension notification is dropped.

```go
// HandleExtensionMethod handles ACP extension methods (names starting with "_").
func (a MyAgent) HandleExtensionMethod(ctx context.Context, method string, params json.RawMessage) (any, error) {
    switch method {
    case "_example.com/hello":
        var p struct {
            Name string `json:"name"`
        }
        if err := json.Unmarshal(params, &p); err != nil {
            return nil, err
        }
        return map[string]any{"greeting": "hello " + p.Name}, nil
    default:
        return nil, acp.NewMethodNotFound(method)
    }
}
```

> Note: Per the ACP spec, unknown extension notifications should be ignored.
> This SDK suppresses noisy logs for unhandled extension notifications that return
> “Method not found”.

`acp.ExtensionMux` does that switch for you, with typed params and results.
`acp.Handle` registers a request method and `acp.Notify` a notification
method; each decodes the params into its type with `encoding/json`, answering
`-32602` when they do not fit. A request for a name not registered with
`Handle` is answered `-32601`, and a notification for a name not registered
with `Notify` is ignored. A name without the `_` prefix, or one already
registered, panics, as a conflicting `http.ServeMux` pattern does.

```go
type MuxAgent struct {
    acp.ExtensionMux
    // ...
}

agent := &MuxAgent{}
acp.Handle(&agent.ExtensionMux, "_example.com/hello", func(ctx context.Context, p HelloParams) (HelloResult, error) {
    return HelloResult{Greeting: "hello " + p.Name}, nil
})
acp.Notify(&agent.ExtensionMux, "_example.com/progress", func(ctx context.Context, p Progress) error {
    return nil
})
```

The mux tells a notification from a request with `acp.IsNotification(ctx)`,
which is true in the context the connection makes for a received
notification; a handler of your own can use it too. `Methods()` lists the
registered names.

#### Calling extension methods

From either side, use `CallExtension` / `NotifyExtension` on the connection.

```go
raw, err := conn.CallExtension(ctx, "_example.com/hello", map[string]any{"name": "world"})
if err != nil {
    return err
}

var resp struct {
    Greeting string `json:"greeting"`
}
if err := json.Unmarshal(raw, &resp); err != nil {
    return err
}

if err := conn.NotifyExtension(ctx, "_example.com/progress", map[string]any{"pct": 50}); err != nil {
    return err
}
```

#### Advertising extension support via `_meta`

ACP uses the `_meta` field inside capability objects as the negotiation/advertising
surface for extensions.

- Client -> Agent: `InitializeRequest.ClientCapabilities.Meta`
- Agent -> Client: `InitializeResponse.AgentCapabilities.Meta`

The ACP specification says the root-level `_meta` keys `traceparent`, `tracestate`,
and `baggage` SHOULD be reserved, for interop with MCP implementations and
OpenTelemetry tooling. W3C Trace Context defines the first two; W3C Baggage
defines `baggage`.

`acp.AdvertiseMeta` returns a copy of a `_meta` object with your features set
under your namespace, and `acp.TraceMeta` returns the reserved trace keys,
leaving out the empty ones:

```go
caps := acp.AgentCapabilities{
    Meta: acp.AdvertiseMeta(nil, "zed.dev", map[string]any{"workspace": true, "fileNotifications": true}),
}
params.Meta = acp.TraceMeta(traceparent, tracestate, "")
```

### Testing with acptest

Package `acptest` stands up a connection pair for a test, with no pipes to
wire by hand. `acptest.Connect(t, agent, client)` joins the two sides through
the public constructors and closes and waits for both when the test ends, so
the pair holds no goroutine past it; a handler that ignores its context fails
the test after 10 seconds, with what still runs, rather than hanging it. It
also provides:

- `ScriptAgent`, which plays scripted prompt turns, each its updates and then
  its stop reason, with a `Usage` when set; or an `Err` instead; or, with
  `WaitCancel`, waiting for the client's `session/cancel` and answering
  `cancelled`;
- `MemoryClient`, which serves files from memory, with 1-based `line` and
  `limit`, answers permission requests and, through `SetElicitation`,
  elicitations from a script, a form's answer checked against the form, and
  serves terminals through its `Terminals`, such as a `*clienthost.Terminals`
  with a scripted `Command`;
- `Recorder`, which keeps each frame a side writes or reads, `Sent` or
  `Received`, in canonical `encoding/json` form, for goldens, and whose
  `Wait(ctx, n)` waits for `n` frames; `ConnectRecorded` records the client
  side of a pair into one;
- `ValidateGolden(t, rec.Records())`, which decodes each recorded frame into
  its method's type, a response by its request's wherever that request is
  among the records, and fails the test on one the schema's `Validate`
  refuses, an unknown method, or a response to no recorded request;
- `ReadRecords`, `WriteRecords` and `Decode[T]`, which read, write and decode
  records one JSON object a line, the form `agentproc.Transcript` writes; and
  `Golden(t, path, records)`, which compares records with a golden file in
  canonical form, rewrites it under `ACPTEST_UPDATE=1`, and otherwise writes
  the records to the test's artifact directory and names the first that
  differs;
- `ReplayAgent`, which plays a recorded session's agent side to a client under
  test, each recorded answer under the live request's id and the agent's own
  requests under fresh ids, and reports where the client departed from the
  recording; `ConnectReplay` joins one to a client, and fails the test on such
  a departure (`ExampleReplayAgent`);
- `Abs`, which makes a fixture path such as `/work/a.txt` absolute on the
  host, as the SDK requires of a path it receives.

```go
func TestPrompt(t *testing.T) {
    agent := &acptest.ScriptAgent{Turns: []acptest.Turn{{
        Updates:    []acp.SessionUpdate{acp.UpdateAgentMessageText("hello")},
        StopReason: acp.StopReasonEndTurn,
    }}}
    client := &acptest.MemoryClient{}
    pair := acptest.Connect(t, agent, client)
    ctx := t.Context()
    if _, err := pair.Client.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
        t.Fatal(err)
    }
    sess, err := pair.Client.NewSession(ctx, acp.NewSessionRequest{Cwd: acptest.Abs("/work")})
    if err != nil {
        t.Fatal(err)
    }
    if _, err := pair.Client.Prompt(ctx, acp.PromptRequest{SessionId: sess.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock("hi")}}); err != nil {
        t.Fatal(err)
    }
    // client.Updates() holds the agent's one update.
}
```

### Study a Production Implementation

For a complete, production‑ready integration, see the
[Gemini CLI Agent](https://github.com/google-gemini/gemini-cli) which exposes an
ACP interface. The Go example client `example/gemini` demonstrates connecting
to it via stdio.

## Stability

Package `acp` keeps Go's compatibility promise for `v1`: a minor release adds
API and removes none. That holds for the names the ACP unstable schema gave it
too. They are frozen in `schema/unstable-v1.json` and stay as they are,
whatever later unstable schemas change or drop. Each carries a `Deprecated:`
notice that says so, so staticcheck's SA1019 reports a use of one. Implementing
`acp.AgentExperimental` or `acp.ClientExperimental` is one such use.

What the unstable schema adds after that goes to package `unstable`. Like
`golang.org/x/exp`, it is not subject to the compatibility promise: a name
there may change or go away in a minor release when the unstable schema changes
it, and `make api` lists those changes without gating them.

- A new definition is a type there, under its schema key, with its
  constructors.
- A new method is its types and its method-name constant. `acp`'s dispatch
  answers a received one as a method it does not know.
- A new variant of a stable union is a `Decode<Union><Variant>` function,
  which decodes the payload `acp`'s union keeps in its `Unknown`.
- A new field of an `acp` type is not emitted.

When a definition stabilizes, it moves to `acp` under the same name. The policy
is [0039-MADR-five-adopter-additions-on-schema-1-25-0.md](docs/decisions/0039-MADR-five-adopter-additions-on-schema-1-25-0.md).

Package `remote` is outside the promise too. The RFD it implements is Active,
not Completed, so a name there may change or go away in a minor release when
the RFD changes, and `make api` lists its changes without gating them
([0043-MADR-remote-http-and-websocket-transport.md](docs/decisions/0043-MADR-remote-http-and-websocket-transport.md)).
The packages the promise covers do not import it.

## Resources

- [Go package docs](https://pkg.go.dev/github.com/maccavelli/acp-go-sdk2)
- [Examples (Go)](https://github.com/maccavelli/acp-go-sdk2/tree/main/example)
- [Protocol Documentation](https://agentclientprotocol.com)
- [Agent Client Protocol GitHub Repository](https://github.com/agentclientprotocol/agent-client-protocol)
- [Documentation index](docs/README.md)

## License

Apache 2.0. See [LICENSE](./LICENSE).
