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
		// session/load, for each. Closing, "The Agent MUST cancel any ongoing
		// work for that session as if session/cancel had been called"
		// (session-setup); the client's pending requests for the session are
		// answered as on session/cancel, once the agent answers with success
		// (0019-PLAN-behaviour-fixes.md, B4).
		{Side: sideAgent, Wire: "session/resume", Capability: "sessionCapabilities.resume"},
		{Side: sideAgent, Wire: "session/close", Capability: "sessionCapabilities.close", ClosesSession: true, SessionKey: keySessionID},
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
	// The properties whose omission, null, and value mean unchanged, cleared,
	// and replaced, which the schema states only in prose
	// (0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md, issue #5).
	PatchFields: []PatchRule{
		{Definition: defCompaction, Property: "summary", Source: srcCompactionPatch},
		{Definition: defCompaction, Property: "error", Source: srcCompactionPatch},
		{Definition: defCompaction, Property: keyMeta, Source: srcCompactionPatch},
		{Definition: defSessionInfo, Property: keyTitle, Source: srcSessionInfoClear},
		{Definition: defSessionInfo, Property: "updatedAt", Source: srcSessionInfoClear},
		{Definition: defSubagent, Property: keyTitle, Source: srcSubagentPatch},
		{Definition: defSubagent, Property: "description", Source: srcSubagentPatch},
		{Definition: defSubagent, Property: "capabilities", Source: srcSubagentPatch},
		{Definition: defSubagent, Property: "state", Source: srcSubagentPatch},
		{Definition: defSubagent, Property: keyMeta, Source: srcSubagentPatch},
		{Definition: defMessage, Property: "content", Source: srcMessageContent},
		{Definition: defMessage, Property: keyMeta, Source: srcMessageMeta},
	},
	// The optional raw properties whose explicit null the schema keeps apart
	// from omission (0019-PLAN-generator-hardening.md, G3).
	NullRetained: []PatchRule{
		{Definition: "UnstableMcpError", Property: "data", Source: srcMcpErrorData},
	},
	// The typed maps whose key order the protocol gives meaning
	// (0022-MADR-keep-elicitation-property-order-before-v1-15-0.md).
	KeyOrders: []OrderRule{
		{Definition: "ElicitationSchema", Property: "properties", Field: "PropertyOrder", Source: srcFormOrder},
	},
	// helpers.go builds these with TextBlock, ToolContent, and the like.
	HandWrittenHelpers: []string{"ContentBlock", defSessionUpdate, "ToolCallContent"},
	// The properties the protocol requires to be absolute paths, which the
	// schema states only in prose (0015-MADR-validate-absolute-protocol-paths.md).
	AbsolutePaths: []PathRule{
		{Definition: defNewSession, Property: keyCwd, Source: srcCwd},
		{Definition: defNewSession, Property: keyRoots, Source: srcRoots},
		{Definition: defLoadSession, Property: keyCwd, Source: srcCwd},
		{Definition: defLoadSession, Property: keyRoots, Source: srcRoots},
		{Definition: defResumeSession, Property: keyCwd, Source: srcCwd},
		{Definition: defResumeSession, Property: keyRoots, Source: srcRoots},
		{Definition: defForkSession, Property: keyCwd, Source: srcCwd},
		{Definition: defForkSession, Property: keyRoots, Source: srcRoots},
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
	// The properties and variants the negotiated capabilities gate, which
	// the SDK checks on what each side sends and receives
	// (0017-MADR-remediate-acp-v1-conformance-violations.md, G1).
	CapabilityRules: []CapabilityRule{
		{Side: sideAgent, Definition: defNewSession, Property: keyRoots, Capability: capRoots, Source: srcRootsGate, Kind: GateNonEmpty},
		{Side: sideAgent, Definition: defLoadSession, Property: keyRoots, Capability: capRoots, Source: srcRootsGate, Kind: GateNonEmpty},
		{Side: sideAgent, Definition: defResumeSession, Property: keyRoots, Capability: capRoots, Source: srcRootsGate, Kind: GateNonEmpty},
		{Side: sideAgent, Definition: defForkSession, Property: keyRoots, Capability: capRoots, Source: srcRootsGate, Kind: GateNonEmpty},
		{Side: sideAgent, Definition: defMcpServer, Variant: "http", Capability: "mcpCapabilities.http", Source: srcMcp, Kind: GateVariant},
		{Side: sideAgent, Definition: defMcpServer, Variant: "sse", Capability: "mcpCapabilities.sse", Source: srcMcp, Kind: GateVariant},
		// Unstable: the schema's capability for the acp transport.
		{Side: sideAgent, Definition: defMcpServer, Variant: "acp", Capability: "mcpCapabilities.acp", Source: srcMcp, Kind: GateVariant},
		{Side: sideAgent, Definition: defPrompt, Property: keyPrompt, Variant: "image", Capability: "promptCapabilities.image", Source: srcPrompt, Kind: GateVariant},
		{Side: sideAgent, Definition: defPrompt, Property: keyPrompt, Variant: "audio", Capability: "promptCapabilities.audio", Source: srcPrompt, Kind: GateVariant},
		{Side: sideAgent, Definition: defPrompt, Property: keyPrompt, Variant: "resource", Capability: "promptCapabilities.embeddedContext", Source: srcPrompt, Kind: GateVariant},
		{Side: sideAgent, Definition: "AuthenticateRequest", Property: "methodId", Variant: "terminal", Source: srcAuthenticate, Kind: GateNotTerminalAuthMethod},
		{Side: sideClient, Definition: defElicitation, Variant: "form", Capability: "elicitation.form", Source: srcElicitation, Kind: GateVariant},
		{Side: sideClient, Definition: defElicitation, Variant: "url", Capability: "elicitation.url", Source: srcElicitation, Kind: GateVariant},
		// A mode the schema does not list is one no capability advertises
		// (0019-MADR-remediate-debugging-pass-findings.md, decision 6).
		{Side: sideClient, Definition: defElicitation, Capability: "elicitation", Source: srcElicitation, Kind: GateUndeclaredVariant},
		{Side: sideClient, Definition: defSessionUpdate, Variant: "notice", Capability: "session.notices", Source: srcSessionUpdate, Kind: GateVariant},
		{Side: sideClient, Definition: defSessionUpdate, Variant: "compaction_update", Capability: "session.compaction", Source: srcSessionUpdate, Kind: GateVariant},
		{Side: sideClient, Definition: defSessionUpdate, Variant: "compaction_summary_chunk", Capability: "session.compaction", Source: srcSessionUpdate, Kind: GateVariant},
		{Side: sideClient, Definition: "SessionConfigOption", Variant: "boolean", Capability: "session.configOptions.boolean", Source: srcBooleanOption, Kind: GateVariant},
		{Side: sideClient, Definition: "InitializeResponse", Property: "authMethods", Variant: "terminal", Capability: "auth.terminal", Source: srcTerminalAuth, Kind: GateVariant},
	},
}

// The definitions several rules name.
const (
	defNewSession    = "NewSessionRequest"
	defLoadSession   = "LoadSessionRequest"
	defResumeSession = "ResumeSessionRequest"
	defForkSession   = "UnstableForkSessionRequest"
	defMcpServer     = "McpServer"
	defPrompt        = "PromptRequest"
	defSessionUpdate = "SessionUpdate"
	defElicitation   = "CreateElicitationRequest"
	defCompaction    = "CompactionUpdate"
	defSubagent      = "SubagentUpdate"
	defMessage       = "SessionMessage"
	defSessionInfo   = "SessionInfoUpdate"
)

// The property keys several rules name.
const (
	keyCwd       = "cwd"
	keyRoots     = "additionalDirectories"
	keyPath      = "path"
	keySessionID = "sessionId"
	keyPrompt    = "prompt"
	keyMeta      = "_meta"
	keyTitle     = "title"
)

// The schema sentences the patch and null-retained rules follow.
const (
	// srcCompactionPatch is from the schema, CompactionUpdate.
	srcCompactionPatch = "`summary`, `error`, and `_meta` have patch semantics: omission leaves the stored value unchanged, `null` clears it, and a concrete value replaces it."
	// srcSessionInfoClear is from the schema, SessionInfoUpdate's title and updatedAt.
	srcSessionInfoClear = "Set to null to clear."
	// srcSubagentPatch is from the schema, SubagentUpdate.
	srcSubagentPatch = "Omitted patch fields keep their previous values; `null` clears them."
	// srcMessageContent is from the schema, SessionMessage's content.
	srcMessageContent = "Omitted leaves content unchanged; `null` or `[]` clears it."
	// srcMessageMeta is from the schema, SessionMessage's _meta.
	srcMessageMeta = "Omitted leaves metadata unchanged; `null` removes it."
	// srcMcpErrorData is from the unstable schema, McpError's data.
	srcMcpErrorData = "Optional error data; explicit null is retained."
)

// srcFormOrder is the reason the key-order rule follows: the schema states no
// order (0022-MADR-keep-elicitation-property-order-before-v1-15-0.md).
const srcFormOrder = "An elicitation schema is a form, whose fields its author orders (GitHub issue #6)."

// The specification sentences the absolute-path rules follow.
const (
	// srcAllPaths is from <https://agentclientprotocol.com/protocol/overview>.
	srcAllPaths = "All file paths in the protocol MUST be absolute."
	// srcCwd is from <https://agentclientprotocol.com/protocol/session-setup>, working directory.
	srcCwd = "The cwd ... MUST be an absolute path."
	// srcRoots is from <https://agentclientprotocol.com/protocol/session-setup>, additional workspace roots.
	srcRoots = "each additionalDirectories entry MUST be an absolute path"
)

// capRoots is the capability that gates additionalDirectories.
const capRoots = "sessionCapabilities.additionalDirectories"

// The specification sentences the capability rules follow.
const (
	// srcRootsGate is from <https://agentclientprotocol.com/protocol/session-setup>, additional workspace roots.
	srcRootsGate = "Clients MUST only send additionalDirectories when the Agent advertises sessionCapabilities.additionalDirectories"
	// srcMcp is from <https://agentclientprotocol.com/protocol/session-setup>, MCP servers.
	srcMcp = "Before using HTTP or SSE transports, Clients MUST verify the Agent's capabilities"
	// srcPrompt is from <https://agentclientprotocol.com/protocol/prompt-turn>.
	srcPrompt = "Clients MUST restrict types of content according to the Prompt Capabilities established during initialization"
	// srcAuthenticate is from <https://agentclientprotocol.com/protocol/authentication>.
	srcAuthenticate = "the Client MUST NOT send an authenticate request for a terminal method"
	// srcElicitation is from the elicitation page.
	srcElicitation = "Agents MUST NOT request a mode the Client has not advertised"
	// srcSessionUpdate is from the schema, Notice and the compaction updates.
	srcSessionUpdate = "Agents MUST only send this update when the Client advertised"
	// srcBooleanOption is from <https://agentclientprotocol.com/protocol/session-config-options>.
	srcBooleanOption = "Agents MUST NOT include type: \"boolean\" options in configOptions payloads unless the Client advertised support"
	// srcTerminalAuth is from the schema, AuthMethodTerminal.
	srcTerminalAuth = "Agents MUST advertise this method only when the client enabled its terminal authentication capability"
)
