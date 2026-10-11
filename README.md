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
- `go run ./example/claude-code` demonstrates bridging to Claude Code.
- `go run ./example/client` connects to a running agent and streams a sample turn.
- `go run ./example/gemini` bridges to the Gemini CLI in ACP mode (flags: -model, -sandbox, -yolo, -debug, -gemini /path/to/gemini).

You can watch the interaction by running `go run ./example/client` locally.

### Explore the API

Browse the Go package docs on pkg.go.dev for detailed API documentation:

- <https://pkg.go.dev/github.com/maccavelli/acp-go-sdk2>

If you're building an [Agent](https://agentclientprotocol.com/protocol/overview#agent):

- Implement the `acp.Agent` interface (and optionally `acp.AgentLoader` for `session/load`).
- Create a connection with `acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)`.
- Send updates and make client requests using the returned connection.

If you're building a [Client](https://agentclientprotocol.com/protocol/overview#client):

- Implement the `acp.Client` interface. It includes the file-system and terminal
  methods, such as `ReadTextFile` and `CreateTerminal`.
- Launch or connect to your Agent process (stdio), then create a connection with
  `acp.NewClientSideConnection(client, stdin, stdout)`.
- Call `Initialize`, `NewSession`, and `Prompt` to run a turn and stream updates.

Helper constructors are provided to reduce boilerplate when working with union types:

- Content blocks: `acp.TextBlock`, `acp.ImageBlock`, `acp.AudioBlock`,
  `acp.ResourceLinkBlock`, `acp.ResourceBlock`.
- Tool content: `acp.ToolContent`, `acp.ToolDiffContent`, `acp.ToolTerminalRef`.
- Tool calls: `acp.StartToolCall`, `acp.UpdateToolCall`, and their options.
  `rawInput` and `rawOutput` are raw JSON, so `acp.WithStartRawInput`,
  `acp.WithStartRawOutput`, `acp.WithUpdateRawInput`, `acp.WithUpdateRawOutput`,
  and the content of `acp.StartEditToolCall` take a `json.RawMessage`.
- Utility: `acp.Ptr[T]` for pointer fields in request/update structs.

### Wire encoding

The SDK encodes and decodes messages with `encoding/json` (v1). The generated types
follow the schema:

- Encoding a message whose required union has no variant set fails with an error
  that names the union, such as `RequestPermissionOutcome: no variant set`.
- A required array or map left nil is sent as `[]` or `{}`, not `null`.
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
  `IdleStateUpdateNone`. Otherwise it decodes into the one variant whose
  required keys it carries. When none or several fit, it is kept in `Unknown`;
  a union with a catch-all instead fails with an error such as
  `StateUpdate: missing discriminator "state"`.
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
  A required raw value, such as a response's `result`, keeps `null`.
- Decoding a struct fails when a required key is absent, naming the type and
  the property, such as `PromptRequest: missing required property "sessionId"`;
  the connection answers such a request with `-32602`.
- A property the schema marks `x-deserialize-default-on-error` takes its schema
  default, or its zero value, when its value is malformed, and an array marked
  `x-deserialize-skip-invalid-items` drops the items that fail to decode, as
  ACP's Rust implementation does.
- `Validate` checks what the schema states, at every depth: a union has exactly
  one variant, or `Unknown`, set; a string meets its `minLength`; every nested
  value, slice element, and map value validates. A present empty string passes.
- A decode error names the union and, for a variant, the discriminator value or
  the variant, then the cause, such as `ContentBlock: type "text": ...`.

The decisions behind these rules are
[0009-MADR-fix-generated-output-on-json-v1.md](docs/decisions/0009-MADR-fix-generated-output-on-json-v1.md),
[0011-MADR-harden-code-generator-engine.md](docs/decisions/0011-MADR-harden-code-generator-engine.md),
and
[0012-MADR-schema-fidelity-framework.md](docs/decisions/0012-MADR-schema-fidelity-framework.md).

### Extension methods

ACP supports **extension methods** for custom JSON-RPC methods whose names start with `_`.
Use them to add functionality without conflicting with future ACP versions.

#### Handling inbound extension methods

Implement `acp.ExtensionMethodHandler` on your Agent or Client. Your handler will be
invoked for any incoming method starting with `_`.

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
