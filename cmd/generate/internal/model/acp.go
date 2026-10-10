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
	Validate:           []ValidateRule{{Type: "ToolCallUpdate", Prop: "toolCallId"}},
}
