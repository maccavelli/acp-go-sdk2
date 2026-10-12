package acp

import "encoding/json"

const (
	blockTypeText         = "text"
	blockTypeImage        = "image"
	blockTypeAudio        = "audio"
	blockTypeResourceLink = "resource_link"
	blockTypeResource     = "resource"
	blockTypeContent      = "content"
	blockTypeDiff         = "diff"
	blockTypeTerminal     = "terminal"
	rawInputPathKey       = "path"
	rawInputContentKey    = "content"
)

// TextBlock constructs a text content block.
func TextBlock(text string) ContentBlock {
	return ContentBlock{Text: &ContentBlockText{
		Text: text,
		Type: blockTypeText,
	}}
}

// ImageBlock constructs an inline image content block with base64-encoded data.
func ImageBlock(data string, mimeType string) ContentBlock {
	return ContentBlock{Image: &ContentBlockImage{
		Data:     data,
		MimeType: mimeType,
		Type:     blockTypeImage,
	}}
}

// AudioBlock constructs an inline audio content block with base64-encoded data.
func AudioBlock(data string, mimeType string) ContentBlock {
	return ContentBlock{Audio: &ContentBlockAudio{
		Data:     data,
		MimeType: mimeType,
		Type:     blockTypeAudio,
	}}
}

// ResourceLinkBlock constructs a resource_link content block with a name and URI.
func ResourceLinkBlock(name string, uri string) ContentBlock {
	return ContentBlock{ResourceLink: &ContentBlockResourceLink{
		Name: name,
		Type: blockTypeResourceLink,
		Uri:  uri,
	}}
}

// ResourceBlock wraps an embedded resource as a content block.
func ResourceBlock(res EmbeddedResourceResource) ContentBlock {
	return ContentBlock{Resource: &ContentBlockResource{
		Resource: res,
		Type:     blockTypeResource,
	}}
}

// ToolContent wraps a content block as tool-call content.
func ToolContent(block ContentBlock) ToolCallContent {
	return ToolCallContent{Content: &ToolCallContentContent{
		Content: block,
		Type:    blockTypeContent,
	}}
}

// ToolDiffContent constructs a diff tool-call content. If oldText is omitted, the field is left empty.
// The path must be absolute: sending fails otherwise (0015-MADR-validate-absolute-protocol-paths.md).
func ToolDiffContent(path string, newText string, oldText ...string) ToolCallContent {
	var o *string
	if len(oldText) > 0 {
		o = &oldText[0]
	}
	return ToolCallContent{Diff: &ToolCallContentDiff{
		NewText: newText,
		OldText: o,
		Path:    path,
		Type:    blockTypeDiff,
	}}
}

// ToolTerminalRef constructs a terminal reference tool-call content.
func ToolTerminalRef(terminalID string) ToolCallContent {
	return ToolCallContent{Terminal: &ToolCallContentTerminal{
		TerminalId: TerminalId(terminalID),
		Type:       blockTypeTerminal,
	}}
}

// Ptr returns a pointer to v.
//
//go:fix inline
func Ptr[T any](v T) *T {
	return new(v)
}

// UpdateUserMessage constructs a user_message_chunk update with the given content.
func UpdateUserMessage(content ContentBlock) SessionUpdate {
	return SessionUpdate{UserMessageChunk: &SessionUpdateUserMessageChunk{Content: content}}
}

// UpdateUserMessageText constructs a user_message_chunk update from text.
func UpdateUserMessageText(text string) SessionUpdate {
	return UpdateUserMessage(TextBlock(text))
}

// UpdateAgentMessage constructs an agent_message_chunk update with the given content.
func UpdateAgentMessage(content ContentBlock) SessionUpdate {
	return SessionUpdate{AgentMessageChunk: &SessionUpdateAgentMessageChunk{Content: content}}
}

// UpdateAgentMessageText constructs an agent_message_chunk update from text.
func UpdateAgentMessageText(text string) SessionUpdate {
	return UpdateAgentMessage(TextBlock(text))
}

// UpdateAgentThought constructs an agent_thought_chunk update with the given content.
func UpdateAgentThought(content ContentBlock) SessionUpdate {
	return SessionUpdate{AgentThoughtChunk: &SessionUpdateAgentThoughtChunk{Content: content}}
}

// UpdateAgentThoughtText constructs an agent_thought_chunk update from text.
func UpdateAgentThoughtText(text string) SessionUpdate {
	return UpdateAgentThought(TextBlock(text))
}

// UpdatePlan constructs a plan update with the provided entries.
func UpdatePlan(entries ...PlanEntry) SessionUpdate {
	return SessionUpdate{Plan: &SessionUpdatePlan{Entries: entries}}
}

// ToolCallStartOpt modifies a tool_call start update before it is built.
type ToolCallStartOpt func(tc *SessionUpdateToolCall)

// StartToolCall constructs a tool_call update with required fields and applies optional modifiers.
func StartToolCall(id ToolCallId, title string, opts ...ToolCallStartOpt) SessionUpdate {
	tc := SessionUpdateToolCall{
		Title:      title,
		ToolCallId: id,
	}
	for _, opt := range opts {
		opt(&tc)
	}
	return SessionUpdate{ToolCall: &tc}
}

// WithStartKind sets the kind for a tool_call start update.
func WithStartKind(k ToolKind) ToolCallStartOpt {
	return func(tc *SessionUpdateToolCall) {
		tc.Kind = k
	}
}

// WithStartStatus sets the status for a tool_call start update.
func WithStartStatus(s ToolCallStatus) ToolCallStartOpt {
	return func(tc *SessionUpdateToolCall) {
		tc.Status = s
	}
}

// WithStartContent sets the initial content for a tool_call start update.
func WithStartContent(c []ToolCallContent) ToolCallStartOpt {
	return func(tc *SessionUpdateToolCall) {
		tc.Content = c
	}
}

// WithStartLocations sets file locations and, if a single path is provided and rawInput is empty, mirrors it as rawInput.path.
// Each path must be absolute: sending fails otherwise (0015-MADR-validate-absolute-protocol-paths.md).
func WithStartLocations(l []ToolCallLocation) ToolCallStartOpt {
	return func(tc *SessionUpdateToolCall) {
		tc.Locations = l
		if len(l) == 1 && l[0].Path != "" {
			tc.RawInput = withRawPath(tc.RawInput, l[0].Path)
		}
	}
}

// editRawInput is {"content": content, "path": path}, keys in the order
// encoding/json writes a map; content is written as given.
func editRawInput(path string, content json.RawMessage) json.RawMessage {
	if len(content) == 0 {
		content = json.RawMessage("null")
	}
	p, err := json.Marshal(path)
	if err != nil { // a string always encodes; kept for errcheck
		return nil
	}
	return json.RawMessage(`{"` + rawInputContentKey + `":` + string(content) + `,"` + rawInputPathKey + `":` + string(p) + `}`)
}

// withRawPath returns raw with a path key set to p when raw is empty, null, or
// a JSON object without one; any other raw input is returned unchanged. The
// other keys keep their values byte for byte. A null raw input is an absent
// one, as ACP v1 defines it.
func withRawPath(raw json.RawMessage, p string) json.RawMessage {
	m := map[string]json.RawMessage{}
	if len(raw) > 0 && string(raw) != jsonNull {
		if err := json.Unmarshal(raw, &m); err != nil || m == nil {
			return raw
		}
	}
	if _, ok := m[rawInputPathKey]; ok {
		return raw
	}
	v, err := json.Marshal(p)
	if err != nil {
		return raw
	}
	m[rawInputPathKey] = v
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}

// WithStartRawInput sets rawInput for a tool_call start update.
func WithStartRawInput(v json.RawMessage) ToolCallStartOpt {
	return func(tc *SessionUpdateToolCall) {
		tc.RawInput = v
	}
}

// WithStartRawOutput sets rawOutput for a tool_call start update.
func WithStartRawOutput(v json.RawMessage) ToolCallStartOpt {
	return func(tc *SessionUpdateToolCall) {
		tc.RawOutput = v
	}
}

// ToolCallUpdateOpt modifies a tool_call_update before it is built.
type ToolCallUpdateOpt func(tu *SessionToolCallUpdate)

// UpdateToolCall constructs a tool_call_update with the given ID and applies optional modifiers.
func UpdateToolCall(id ToolCallId, opts ...ToolCallUpdateOpt) SessionUpdate {
	tu := SessionToolCallUpdate{ToolCallId: id}
	for _, opt := range opts {
		opt(&tu)
	}
	return SessionUpdate{ToolCallUpdate: &tu}
}

// WithUpdateTitle sets the title for a tool_call_update.
func WithUpdateTitle(t string) ToolCallUpdateOpt {
	return func(tu *SessionToolCallUpdate) {
		tu.Title = new(t)
	}
}

// WithUpdateKind sets the kind for a tool_call_update.
func WithUpdateKind(k ToolKind) ToolCallUpdateOpt {
	return func(tu *SessionToolCallUpdate) {
		tu.Kind = new(k)
	}
}

// WithUpdateStatus sets the status for a tool_call_update.
func WithUpdateStatus(s ToolCallStatus) ToolCallUpdateOpt {
	return func(tu *SessionToolCallUpdate) {
		tu.Status = new(s)
	}
}

// WithUpdateContent replaces the content collection for a tool_call_update.
func WithUpdateContent(c []ToolCallContent) ToolCallUpdateOpt {
	return func(tu *SessionToolCallUpdate) {
		tu.Content = c
	}
}

// WithUpdateLocations replaces the locations collection for a tool_call_update.
// Each path must be absolute: sending fails otherwise (0015-MADR-validate-absolute-protocol-paths.md).
func WithUpdateLocations(l []ToolCallLocation) ToolCallUpdateOpt {
	return func(tu *SessionToolCallUpdate) {
		tu.Locations = l
	}
}

// WithUpdateRawInput sets rawInput for a tool_call_update.
func WithUpdateRawInput(v json.RawMessage) ToolCallUpdateOpt {
	return func(tu *SessionToolCallUpdate) {
		tu.RawInput = v
	}
}

// WithUpdateRawOutput sets rawOutput for a tool_call_update.
func WithUpdateRawOutput(v json.RawMessage) ToolCallUpdateOpt {
	return func(tu *SessionToolCallUpdate) {
		tu.RawOutput = v
	}
}

// StartReadToolCall constructs a 'tool_call' update for reading a file: kind=read, status=pending, locations=[{path}], rawInput={path}.
// The path must be absolute: sending fails otherwise (0015-MADR-validate-absolute-protocol-paths.md).
func StartReadToolCall(id ToolCallId, title string, path string, opts ...ToolCallStartOpt) SessionUpdate {
	base := []ToolCallStartOpt{WithStartKind(ToolKindRead), WithStartStatus(ToolCallStatusPending), WithStartLocations([]ToolCallLocation{{Path: path}}), WithStartRawInput(withRawPath(nil, path))}
	base = append(base, opts...)
	return StartToolCall(id, title, base...)
}

// StartEditToolCall constructs a 'tool_call' update for editing content: kind=edit, status=pending, locations=[{path}], rawInput={path, content}.
// content is the JSON of the new content, such as a quoted string; nil sends null.
// The path must be absolute: sending fails otherwise (0015-MADR-validate-absolute-protocol-paths.md).
func StartEditToolCall(id ToolCallId, title string, path string, content json.RawMessage, opts ...ToolCallStartOpt) SessionUpdate {
	base := []ToolCallStartOpt{WithStartKind(ToolKindEdit), WithStartStatus(ToolCallStatusPending), WithStartLocations([]ToolCallLocation{{Path: path}}), WithStartRawInput(editRawInput(path, content))}
	base = append(base, opts...)
	return StartToolCall(id, title, base...)
}
