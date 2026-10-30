// Package remote runs ACP over the network: the remote transport of the RFD
// "Streamable HTTP & WebSocket Transport"
// (https://agentclientprotocol.com/rfds/streamable-http-websocket-transport),
// at its revision of 2026-07-02, as the specification repository holds it at
// commit 2bb9fac (docs/decisions/0043-MADR-remote-http-and-websocket-transport.md).
//
// Like package unstable, this package is not subject to the Go 1
// compatibility promise: the RFD is Active, not Completed, and a name here
// may change or go away in a minor release of this module when the RFD
// changes. make api lists its changes without gating them.
//
// The RFD has two profiles on one endpoint, both of which a client must
// speak and either of which a server may serve:
//
//   - Streamable HTTP: a POST for each message to the agent, and long-lived
//     server-sent-event streams, one for the connection and one for each
//     session, for the messages from it;
//   - WebSocket: a GET upgraded to a socket that carries every message both
//     ways, one text frame each.
//
// Both sides present a remote connection as the byte stream the rest of this
// module reads and writes, one JSON-RPC message a line, as stdio carries it.
// A message received becomes one compact line, and each line written becomes
// one message. So a connection, a serve function, the taps and the transcripts
// work over the network as they do over a child's pipes.
//
// A client calls Dial and joins the Conn it returns to its client:
// acp.NewClientSideConnection(client, conn, conn). A server mounts the Handler
// NewHandler returns on its own http.Server, with acp.ServeAgent as the usual
// serve function, and calls Handler.Close before the server's Shutdown.
// Authentication, TLS, CORS and limits on connections are the server's: the
// RFD layers them on the transport, and Acp-Connection-Id is not a credential.
package remote
