// Package acp is a Go implementation of the Agent Client Protocol (ACP): the
// protocol's types, generated from its schema, and connections that speak
// JSON-RPC 2.0 over line-delimited JSON.
//
// # The two sides
//
// An agent implements [Agent] and serves it with [NewAgentSideConnection];
// a client implements [Client] and serves it with [NewClientSideConnection].
// Each constructor takes the writer to the peer and the reader from it,
// usually a process's stdout and stdin for an agent, and the agent process's
// stdin and stdout for a client. The returned [AgentSideConnection] and
// [ClientSideConnection] have a typed method for each message they send to
// the peer, such as ClientSideConnection.Prompt and
// AgentSideConnection.SessionUpdate.
//
// Reading starts inside the constructor, so a handler can run before the
// constructor returns. An agent or client that keeps its connection
// implements [AgentConnAware] or [ClientConnAware], whose method the
// constructor calls before reading starts.
//
// A client calls Initialize first. Until it completes, every other typed
// method on either side returns an error wrapping [ErrNotInitialized], and
// what initialize agreed then bounds what each side may send.
//
// # How a connection ends
//
// Done is closed when the connection ends: when the peer disconnects or its
// output ends, when Close is called, or when a full notification queue
// closes it under [OverflowCloseConnection], or under [OverflowBlock] past
// its read-on limit. Under OverflowBlock, the default, a full queue pauses
// reading instead. Close ends every call waiting for a response, and every
// handler's context.
//
// # Errors
//
// The connection's errors wrap sentinels, for [errors.Is]:
// [ErrNotInitialized], [ErrTerminalAuthMethod], [ErrPeerDisconnected],
// [ErrConnectionClosed] and [ErrPromptCancelled]. Four error types carry
// details, for [errors.AsType]: [*RequestError], a JSON-RPC error, sent or
// received; [*ValidationError], a message that breaks the schema;
// [*CapabilityError], a message that what initialize agreed does not allow;
// and [*ProtocolVersionError], a protocol version this package does not
// speak. Each typed method's doc names the ones it returns.
//
// # Where to start
//
// ExampleNewClientSideConnection, the README's quickstart, connects a client
// and an agent in one process and initializes. Example_agent and
// Example_client run a whole prompt turn the same way. The example programs
// under example/ run an agent and a client as separate processes, and the
// README describes each.
package acp
