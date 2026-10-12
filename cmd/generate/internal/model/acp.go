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
		// Session setup: Clients MUST NOT call session/load unless the Agent
		// advertises loadSession, so it has its own optional interface.
		{Side: sideAgent, Wire: "session/load", Binding: BindAgentOptional, Capability: "loadSession"},
		// Prompt turn: session/cancel cancels the session's prompt; a
		// client whose session/prompt context ends sends session/cancel.
		{Side: sideAgent, Wire: "session/prompt", PromptTurn: true, SessionKey: "sessionId"},
		{Side: sideAgent, Wire: "session/cancel", CancelsPromptTurn: true, SessionKey: "sessionId"},
		// The client's notification handler reads as what it receives.
		{Side: sideClient, Wire: "session/update", GoName: "SessionUpdate"},

		// Capability gates the schema describes, recorded, not acted on.
		{Side: sideAgent, Wire: "session/list", Capability: "sessionCapabilities.list"},
		{Side: sideAgent, Wire: "session/delete", Capability: "sessionCapabilities.delete"},
		{Side: sideAgent, Wire: "session/resume", Capability: "sessionCapabilities.resume"},
		{Side: sideAgent, Wire: "session/close", Capability: "sessionCapabilities.close"},
		{Side: sideAgent, Wire: "session/fork", Capability: "sessionCapabilities.fork"},
		{Side: sideAgent, Wire: "logout", Capability: "auth.logout"},
		{Side: sideClient, Wire: "fs/read_text_file", Capability: "fs.readTextFile"},
		{Side: sideClient, Wire: "fs/write_text_file", Capability: "fs.writeTextFile"},
		{Side: sideClient, Wire: "terminal/create", Capability: capTerminal},
		{Side: sideClient, Wire: "terminal/output", Capability: capTerminal},
		{Side: sideClient, Wire: "terminal/release", Capability: capTerminal},
		{Side: sideClient, Wire: "terminal/wait_for_exit", Capability: capTerminal},
		{Side: sideClient, Wire: "terminal/kill", Capability: capTerminal},
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

// The property keys several absolute-path rules name.
const (
	keyCwd   = "cwd"
	keyRoots = "additionalDirectories"
	keyPath  = "path"
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
