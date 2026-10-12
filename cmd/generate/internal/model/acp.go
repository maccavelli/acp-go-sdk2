package model

// capTerminal gates every terminal/* method.
const capTerminal = "terminal"

// ACP is the protocol table the generator uses: the behaviour of the Agent
// Client Protocol that its schema does not express. Each rule names the
// specification it follows; Build checks every rule against the schema it
// applies to, and TestProtocolTableAppliesToCommittedSchema requires every
// rule to apply to the schema in the repository.
var ACP = Protocol{
	Interfaces: []Interface{{
		Binding: BindAgentOptional,
		Name:    "AgentLoader",
		Var:     "loader",
		Doc:     []string{"AgentLoader defines optional support for loading sessions. Implement and advertise the capability to enable 'session/load'."},
	}},
	Methods: []MethodRule{
		// Initialization: "The initialize request MUST include the latest
		// protocol version the Client supports"; "If the Agent supports the
		// requested version, it MUST respond with the same version. Otherwise,
		// the Agent MUST respond with the latest version it supports."
		{Side: sideAgent, Wire: "initialize", Initializes: true, VersionKey: "protocolVersion",
			RequestCapabilities: "clientCapabilities", ResponseCapabilities: "agentCapabilities"},
		// Session setup: "Clients MUST NOT attempt to call session/load" unless
		// the Agent advertises loadSession; the SDK enforces it, and it has its
		// own optional interface.
		{Side: sideAgent, Wire: "session/load", Binding: BindAgentOptional, Capability: "loadSession"},
		// Prompt turn: session/cancel cancels the session's prompt; a
		// client whose session/prompt context ends sends session/cancel. The
		// cancelled stop reason "MUST be returned when the client sends a
		// session/cancel notification, even if the cancellation causes
		// exceptions in underlying operations" (schema, StopReason).
		{Side: sideAgent, Wire: "session/prompt", PromptTurn: true, SessionKey: keySessionID, StopKey: "stopReason", StopCancelled: "cancelled"},
		{Side: sideAgent, Wire: "session/cancel", CancelsPromptTurn: true, SessionKey: keySessionID},
		// "The Client MUST respond to all pending session/request_permission
		// requests with the cancelled outcome" when it sends session/cancel
		// (prompt-turn, Cancellation).
		{Side: sideClient, Wire: "session/request_permission", CancelledByPromptCancel: true, SessionKey: keySessionID,
			CancelledKey: "outcome", CancelledVariant: "cancelled"},
		// The client's notification handler reads as what it receives.
		{Side: sideClient, Wire: "session/update", GoName: "SessionUpdate"},

		// Capability gates, enforced: the typed method does not send a gated
		// method the record does not allow, and a received one is refused unless
		// the validation hook accepts it
		// (0017-MADR-remediate-acp-v1-conformance-violations.md, G1).
		// "If sessionCapabilities.list is not present, the Agent does not support
		// listing sessions and Clients MUST NOT attempt to call session/list."
		{Side: sideAgent, Wire: "session/list", Capability: "sessionCapabilities.list"},
		// "Clients MUST NOT attempt to call session/delete."
		{Side: sideAgent, Wire: "session/delete", Capability: "sessionCapabilities.delete"},
		// session/resume and session/close: the session-setup sentence of
		// session/load, for each.
		{Side: sideAgent, Wire: "session/resume", Capability: "sessionCapabilities.resume"},
		{Side: sideAgent, Wire: "session/close", Capability: "sessionCapabilities.close"},
		// Unstable: the schema's capability for session/fork.
		{Side: sideAgent, Wire: "session/fork", Capability: "sessionCapabilities.fork"},
		// "If agentCapabilities.auth.logout is omitted or null, the Agent does
		// not support logout and Clients MUST NOT call it."
		{Side: sideAgent, Wire: "logout", Capability: "auth.logout"},
		// An Agent "MUST NOT attempt to call the corresponding filesystem
		// method" the Client does not advertise.
		{Side: sideClient, Wire: "fs/read_text_file", Capability: "fs.readTextFile"},
		{Side: sideClient, Wire: "fs/write_text_file", Capability: "fs.writeTextFile"},
		// Without the terminal capability an Agent "MUST NOT attempt to call any
		// terminal methods".
		{Side: sideClient, Wire: "terminal/create", Capability: capTerminal},
		{Side: sideClient, Wire: "terminal/output", Capability: capTerminal},
		{Side: sideClient, Wire: "terminal/release", Capability: capTerminal},
		{Side: sideClient, Wire: "terminal/wait_for_exit", Capability: capTerminal},
		{Side: sideClient, Wire: "terminal/kill", Capability: capTerminal},
		// "Agents MUST NOT request a mode the Client has not advertised";
		// elicitation/complete is "sent by the agent when a URL-based
		// elicitation is complete" (schema), so it needs the URL mode.
		{Side: sideClient, Wire: "elicitation/complete", Capability: "elicitation.url"},
	},
	// helpers.go builds these with TextBlock, ToolContent, and the like.
	HandWrittenHelpers: []string{"ContentBlock", "SessionUpdate", "ToolCallContent"},
	// The properties the protocol requires to be absolute paths, which the
	// schema states only in prose (0015-MADR-validate-absolute-protocol-paths.md).
	AbsolutePaths: []PathRule{
		{Definition: "NewSessionRequest", Property: keyCwd, Source: srcCwd},
		{Definition: "NewSessionRequest", Property: keyRoots, Source: srcRoots},
		{Definition: "LoadSessionRequest", Property: keyCwd, Source: srcCwd},
		{Definition: "LoadSessionRequest", Property: keyRoots, Source: srcRoots},
		{Definition: "ResumeSessionRequest", Property: keyCwd, Source: srcCwd},
		{Definition: "ResumeSessionRequest", Property: keyRoots, Source: srcRoots},
		{Definition: "UnstableForkSessionRequest", Property: keyCwd, Source: srcCwd},
		{Definition: "UnstableForkSessionRequest", Property: keyRoots, Source: srcRoots},
		{Definition: "ListSessionsRequest", Property: keyCwd, Source: srcAllPaths},
		{Definition: "SessionInfo", Property: keyCwd, Source: srcCwd},
		{Definition: "SessionInfo", Property: keyRoots, Source: srcRoots},
		{Definition: "McpServerStdio", Property: "command", Source: srcAllPaths},
		{Definition: "ReadTextFileRequest", Property: keyPath, Source: srcAllPaths},
		{Definition: "WriteTextFileRequest", Property: keyPath, Source: srcAllPaths},
		{Definition: "CreateTerminalRequest", Property: keyCwd, Source: srcAllPaths},
		{Definition: "ToolCallLocation", Property: keyPath, Source: srcAllPaths},
		{Definition: "Diff", Property: keyPath, Source: srcAllPaths},
	},
}

// The property keys several rules name.
const (
	keyCwd       = "cwd"
	keyRoots     = "additionalDirectories"
	keyPath      = "path"
	keySessionID = "sessionId"
)

// The specification sentences the absolute-path rules follow.
const (
	// srcAllPaths is from <https://agentclientprotocol.com/protocol/overview>.
	srcAllPaths = "All file paths in the protocol MUST be absolute."
	// srcCwd is from <https://agentclientprotocol.com/protocol/session-setup>, working directory.
	srcCwd = "The cwd ... MUST be an absolute path."
	// srcRoots is from <https://agentclientprotocol.com/protocol/session-setup>, additional workspace roots.
	srcRoots = "each additionalDirectories entry MUST be an absolute path"
)
