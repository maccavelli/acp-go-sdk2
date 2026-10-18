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

## Get Started

### Understand the Protocol

Start by reading the [official ACP documentation](https://agentclientprotocol.com)
to understand the core concepts and protocol specification.

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

The example clients advertise the file-system capability and not the terminal
one, and write only inside the session's working directory. You can watch the
interaction by running `go run ./example/client` locally, answering the
permission request with `1` or `2`.

### Explore the API

Browse the Go package docs on pkg.go.dev for detailed API documentation:

- <https://pkg.go.dev/github.com/maccavelli/acp-go-sdk2>

If you're building an [Agent](https://agentclientprotocol.com/protocol/overview#agent):

- Implement the `acp.Agent` interface (and optionally `acp.AgentLoader` for `session/load`).
- Create a connection with `acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)`.
- Send updates and make client requests using the returned connection. An agent
  that keeps the connection for its handlers implements `acp.AgentConnAware`:
  the constructor calls its `SetAgentConnection` before it reads anything, so a
  handler always finds the connection set. The method only stores it.
  Assigning the returned connection after the constructor instead races a
  handler that is already running.

If you're building a [Client](https://agentclientprotocol.com/protocol/overview#client):

- Implement the `acp.Client` interface. It includes the file-system and terminal
  methods, such as `ReadTextFile` and `CreateTerminal`.
- Launch or connect to your Agent process (stdio), then create a connection with
  `acp.NewClientSideConnection(client, stdin, stdout)`.
- Call `Initialize`, `NewSession`, and `Prompt` to run a turn and stream updates.
  A client that keeps its connection implements `acp.ClientConnAware`, whose
  `SetClientConnection` the constructor calls the same way.

Helper constructors are provided to reduce boilerplate when working with union types:

- Content blocks: `acp.TextBlock`, `acp.ImageBlock`, `acp.AudioBlock`,
  `acp.ResourceLinkBlock`, `acp.ResourceBlock`.
- Tool content: `acp.ToolContent`, `acp.ToolDiffContent`, `acp.ToolTerminalRef`.
- Tool calls: `acp.StartToolCall`, `acp.UpdateToolCall`, and their options.
  `rawInput` and `rawOutput` are raw JSON, so `acp.WithStartRawInput`,
  `acp.WithStartRawOutput`, `acp.WithUpdateRawInput`, `acp.WithUpdateRawOutput`,
  and the content of `acp.StartEditToolCall` take a `json.RawMessage`.
- Utility: `acp.Ptr[T]` for pointer fields in request/update structs.
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
   `*acp.ValidationError` when the error is the SDK's.

A rewrite hook can resolve what a peer sends as a bare name, or translate a
path from another host, such as a Windows client's path for an agent in WSL:

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
the request and the agent's answer as an `acp.Negotiation` when it returns
without error, and `AgentSideConnection` records the same before its answer is
written. `Negotiated()` on either side returns the record and whether there is
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
answered `-32601`. After it, the SDK checks what each side sends and receives
against the capabilities in the record:

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

`Authenticate` with the id of a `terminal` auth method the agent listed returns
an error wrapping `acp.ErrTerminalAuthMethod` and sends nothing, since a
terminal method is run, not authenticated; received, it is answered `-32602`.

`Negotiation.Allows(method)` reports whether the record lets a method be sent,
for an application that shows or hides features: nil, or the
`*acp.CapabilityError`.

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
without writing, and the reader and the writer are closed when they implement
`io.Closer`, which also ends a read in progress. A second `Close` returns nil.
When the peer ends the connection, the same holds, but for the transport,
which is the application's to close. The default overflow policy,
`acp.OverflowCloseConnection`, closes the connection as `Close` does.

Every send returns when its context ends, even while the peer does not read.
A message whose write has begun may still reach the peer.

`acp.WithMaxConcurrentRequests(n)` bounds the received requests whose handlers
run at once. Over the limit, a request is answered at once, `-32603` with
`{"error": "too many concurrent requests", "limit": n}`, and its handler does not
run; the reader never waits for a handler, since a running one may be waiting
for a response only the reader delivers. Notifications and `$/cancel_request`
do not count. The default is no limit.

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
`errors.Is`: a call whose context ended wraps `context.Canceled` or
`context.DeadlineExceeded`; one whose connection ended wraps
`acp.ErrPeerDisconnected`, or `acp.ErrConnectionClosed` after `Close`; one
whose write failed wraps the write's error. Its code is as before. An error the
peer answered wraps nothing, so a peer's `-32800` is not `context.Canceled`.

A malformed message is logged by its fault, its size and, when it has a valid
one, its id, never by its content, which may hold prompt text or file contents.

These rules are decided in
[0017-MADR-remediate-acp-v1-conformance-violations.md](docs/decisions/0017-MADR-remediate-acp-v1-conformance-violations.md)
and
[0019-MADR-remediate-debugging-pass-findings.md](docs/decisions/0019-MADR-remediate-debugging-pass-findings.md).

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

### Study a Production Implementation

For a complete, production‑ready integration, see the
[Gemini CLI Agent](https://github.com/google-gemini/gemini-cli) which exposes an
ACP interface. The Go example client `example/gemini` demonstrates connecting
to it via stdio.

## Resources

- [Go package docs](https://pkg.go.dev/github.com/maccavelli/acp-go-sdk2)
- [Examples (Go)](https://github.com/maccavelli/acp-go-sdk2/tree/main/example)
- [Protocol Documentation](https://agentclientprotocol.com)
- [Agent Client Protocol GitHub Repository](https://github.com/agentclientprotocol/agent-client-protocol)
- [Documentation index](docs/README.md)

## License

Apache 2.0. See [LICENSE](./LICENSE).
