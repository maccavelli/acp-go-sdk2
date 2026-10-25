package acp

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"path/filepath"
	"time"
)

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

	outcomeCancelled    = "cancelled"
	outcomeSelected     = "selected"
	configOptionSelect  = "select"
	configOptionBoolean = "boolean"

	sessionUpdateUserMessage         = "user_message_chunk"
	sessionUpdateAgentMessage        = "agent_message_chunk"
	sessionUpdateAgentThought        = "agent_thought_chunk"
	sessionUpdateToolCall            = "tool_call"
	sessionUpdateToolCallUpdate      = "tool_call_update"
	sessionUpdatePlan                = "plan"
	sessionUpdatePlanUpdate          = "plan_update"
	sessionUpdatePlanRemoved         = "plan_removed"
	sessionUpdateAvailableCommands   = "available_commands_update"
	sessionUpdateCurrentMode         = "current_mode_update"
	sessionUpdateConfigOptions       = "config_option_update"
	sessionUpdateSessionInfo         = "session_info_update"
	sessionUpdateUsage               = "usage_update"
	sessionUpdateNotice              = "notice"
	sessionUpdateCompaction          = "compaction_update"
	sessionUpdateCompactionSummary   = "compaction_summary_chunk"
	sessionUpdateSubagent            = "subagent_update"
	sessionUpdateSessionMessage      = "session_message"
	sessionUpdateSessionMessageChunk = "session_message_chunk"
	sessionUpdateUnknownKind         = "unknown"
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

// roots returns cwd followed by more, in a new slice.
func roots(cwd string, more []string) []string {
	return append([]string{cwd}, more...)
}

// Roots returns the session's root set: cwd, then each of
// additionalDirectories, in order, in a new slice. It checks nothing;
// confining file access to the roots is the agent's, for example with
// os.OpenRoot for each one (0015-MADR-validate-absolute-protocol-paths.md, X1).
func (r *NewSessionRequest) Roots() []string { return roots(r.Cwd, r.AdditionalDirectories) }

// Roots returns the session's root set, as NewSessionRequest.Roots does.
func (r *LoadSessionRequest) Roots() []string { return roots(r.Cwd, r.AdditionalDirectories) }

// Roots returns the session's root set, as NewSessionRequest.Roots does.
func (r *ResumeSessionRequest) Roots() []string { return roots(r.Cwd, r.AdditionalDirectories) }

// Roots returns the session's root set, as NewSessionRequest.Roots does.
func (r *UnstableForkSessionRequest) Roots() []string { return roots(r.Cwd, r.AdditionalDirectories) }

// Roots returns the listed session's root set, as NewSessionRequest.Roots
// does.
func (r *SessionInfo) Roots() []string { return roots(r.Cwd, r.AdditionalDirectories) }

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

// UpdateAvailableCommands constructs an available_commands_update update with
// the agent's commands; nil is sent as [].
func UpdateAvailableCommands(commands []AvailableCommand) SessionUpdate {
	return SessionUpdate{AvailableCommandsUpdate: &SessionAvailableCommandsUpdate{
		AvailableCommands: commands,
		SessionUpdate:     sessionUpdateAvailableCommands,
	}}
}

// UpdateCurrentMode constructs a current_mode_update update naming the
// session's current mode.
func UpdateCurrentMode(id SessionModeId) SessionUpdate {
	return SessionUpdate{CurrentModeUpdate: &SessionCurrentModeUpdate{
		CurrentModeId: id,
		SessionUpdate: sessionUpdateCurrentMode,
	}}
}

// UpdateConfigOptions constructs a config_option_update update with the full
// set of the session's configuration options; nil is sent as [].
func UpdateConfigOptions(opts []SessionConfigOption) SessionUpdate {
	return SessionUpdate{ConfigOptionUpdate: &SessionConfigOptionUpdate{
		ConfigOptions: opts,
		SessionUpdate: sessionUpdateConfigOptions,
	}}
}

// UpdateSessionTitle constructs a session_info_update update with the given
// title patch: PatchValue sets the title, PatchNull clears it, and the zero
// Patch leaves it unchanged. UpdatedAt and Meta are set on the returned
// variant.
func UpdateSessionTitle(title Patch[string]) SessionUpdate {
	return SessionUpdate{SessionInfoUpdate: &SessionSessionInfoUpdate{
		SessionUpdate: sessionUpdateSessionInfo,
		Title:         title,
	}}
}

// UpdateUsage constructs a usage_update update: used is the tokens currently
// in context, size the context window's size. Cost is set on the returned
// variant.
func UpdateUsage(used, size uint64) SessionUpdate {
	return SessionUpdate{UsageUpdate: &SessionUsageUpdate{
		SessionUpdate: sessionUpdateUsage,
		Size:          size,
		Used:          used,
	}}
}

// UpdateNotice constructs a notice update. Sending it fails with a
// *CapabilityError unless the client advertised session.notices, and an
// empty title fails Validate, so it is not sent either. Description and Meta
// are set on the returned variant.
func UpdateNotice(severity NoticeSeverity, title string) SessionUpdate {
	return SessionUpdate{Notice: &SessionUpdateNotice{
		SessionUpdate: sessionUpdateNotice,
		Severity:      severity,
		Title:         title,
	}}
}

// UpdateCompaction constructs a compaction_update update for the compaction
// id. Sending it fails with a *CapabilityError unless the client advertised
// session.compaction. Summary, Error and Meta are set on the returned variant.
func UpdateCompaction(id CompactionId, status CompactionStatus) SessionUpdate {
	return SessionUpdate{CompactionUpdate: &SessionCompactionUpdate{
		CompactionId:  id,
		SessionUpdate: sessionUpdateCompaction,
		Status:        status,
	}}
}

// UpdateCompactionSummary constructs a compaction_summary_chunk update that
// appends content to the compaction id's summary. Sending it fails with a
// *CapabilityError unless the client advertised session.compaction.
func UpdateCompactionSummary(id CompactionId, content ContentBlock) SessionUpdate {
	return SessionUpdate{CompactionSummaryChunk: &SessionUpdateCompactionSummaryChunk{
		CompactionId:  id,
		Content:       content,
		SessionUpdate: sessionUpdateCompactionSummary,
	}}
}

// UpdateCompactionSummaryText constructs a compaction_summary_chunk update
// from text, as UpdateCompactionSummary does with a text block.
func UpdateCompactionSummaryText(id CompactionId, text string) SessionUpdate {
	return UpdateCompactionSummary(id, TextBlock(text))
}

// Kind returns the sessionUpdate value of the variant u sends: the first set
// variant, in the order MarshalJSON checks them, such as "agent_message_chunk"
// or "notice". With no variant set, it returns the "sessionUpdate" of Unknown,
// read as UnmarshalJSON reads it, when that is a non-empty string, and
// "unknown" when Unknown holds anything else. The zero SessionUpdate returns "".
func (u SessionUpdate) Kind() string {
	switch {
	case u.UserMessageChunk != nil:
		return sessionUpdateUserMessage
	case u.AgentMessageChunk != nil:
		return sessionUpdateAgentMessage
	case u.AgentThoughtChunk != nil:
		return sessionUpdateAgentThought
	case u.ToolCall != nil:
		return sessionUpdateToolCall
	case u.ToolCallUpdate != nil:
		return sessionUpdateToolCallUpdate
	case u.Plan != nil:
		return sessionUpdatePlan
	case u.PlanUpdate != nil:
		return sessionUpdatePlanUpdate
	case u.PlanRemoved != nil:
		return sessionUpdatePlanRemoved
	case u.AvailableCommandsUpdate != nil:
		return sessionUpdateAvailableCommands
	case u.CurrentModeUpdate != nil:
		return sessionUpdateCurrentMode
	case u.ConfigOptionUpdate != nil:
		return sessionUpdateConfigOptions
	case u.SessionInfoUpdate != nil:
		return sessionUpdateSessionInfo
	case u.UsageUpdate != nil:
		return sessionUpdateUsage
	case u.Notice != nil:
		return sessionUpdateNotice
	case u.CompactionUpdate != nil:
		return sessionUpdateCompaction
	case u.CompactionSummaryChunk != nil:
		return sessionUpdateCompactionSummary
	case u.SubagentUpdate != nil:
		return sessionUpdateSubagent
	case u.SessionMessage != nil:
		return sessionUpdateSessionMessage
	case u.SessionMessageChunk != nil:
		return sessionUpdateSessionMessageChunk
	case u.Unknown != nil:
		return unknownKind(u.Unknown, "sessionUpdate")
	}
	return ""
}

// unknownKind reads the discriminator key of b as a union's UnmarshalJSON
// reads it, folding keys, the last match winning, and returns "unknown" when
// that is not a non-empty string.
func unknownKind(b json.RawMessage, key string) string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return sessionUpdateUnknownKind
	}
	foldKeys(b, m, key)
	var kind string
	if err := json.Unmarshal(m[key], &kind); err != nil || kind == "" {
		return sessionUpdateUnknownKind
	}
	return kind
}

// Kind returns the type value of the content block b sends: the first set
// variant, in the order MarshalJSON checks them, such as "text" or
// "resource_link". With no variant set, it returns the "type" of Unknown, read
// as UnmarshalJSON reads it, when that is a non-empty string, and "unknown"
// when Unknown holds anything else. The zero ContentBlock returns ""
// (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
func (b ContentBlock) Kind() string {
	switch {
	case b.Text != nil:
		return blockTypeText
	case b.Image != nil:
		return blockTypeImage
	case b.Audio != nil:
		return blockTypeAudio
	case b.ResourceLink != nil:
		return blockTypeResourceLink
	case b.Resource != nil:
		return blockTypeResource
	case b.Unknown != nil:
		return unknownKind(b.Unknown, "type")
	}
	return ""
}

// Kind returns the type value of the tool-call content c sends, "content",
// "diff" or "terminal", as ContentBlock.Kind does for a content block
// (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
func (c ToolCallContent) Kind() string {
	switch {
	case c.Content != nil:
		return blockTypeContent
	case c.Diff != nil:
		return blockTypeDiff
	case c.Terminal != nil:
		return blockTypeTerminal
	case c.Unknown != nil:
		return unknownKind(c.Unknown, "type")
	}
	return ""
}

// Kind returns the outcome value o sends, "cancelled" or "selected", as
// ContentBlock.Kind does for a content block, reading an unknown one's
// "outcome" (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
func (o RequestPermissionOutcome) Kind() string {
	switch {
	case o.Cancelled != nil:
		return outcomeCancelled
	case o.Selected != nil:
		return outcomeSelected
	case o.Unknown != nil:
		return unknownKind(o.Unknown, "outcome")
	}
	return ""
}

// Kind returns the type value the config option o sends, "select" or
// "boolean", as ContentBlock.Kind does for a content block
// (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
func (o SessionConfigOption) Kind() string {
	switch {
	case o.Select != nil:
		return configOptionSelect
	case o.Boolean != nil:
		return configOptionBoolean
	case o.Unknown != nil:
		return unknownKind(o.Unknown, "type")
	}
	return ""
}

// ErrRepeatedCursor is the error AllSessions yields when the agent answers
// a cursor it already gave, so following it would never end
// (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
var ErrRepeatedCursor = errors.New("acp: session/list answered a cursor it already gave")

// errNilConnection is what a helper taking a side connection yields or
// returns when it is given none.
var errNilConnection = errors.New("acp: nil connection")

// AllSessions lists every session c's agent has, from req.Cursor on: it
// calls c.ListSessions, yields each session, and follows NextCursor until the
// agent gives none, or an empty one. A cursor already seen, the starting one
// included, yields ErrRepeatedCursor and stops, so the list is known to be
// incomplete. An error from a call, a *RequestError, ErrNotInitialized or
// the end of ctx among them, is yielded once and stops; so is an error for a
// nil c. Breaking out of the loop sends no further request. Sent to an agent
// that did not advertise sessionCapabilities.list, the first call fails with
// a *CapabilityError and writes nothing. The cursors seen are kept until the
// loop ends, so ctx bounds an agent that answers new cursors forever.
//
// It is a function, not a method of ClientSideConnection: every method of a
// side connection that takes a context is a protocol method
// (0038-PLAN-consumer-helpers-kind-and-agent-refusals.md, step 2's deviation).
func AllSessions(ctx context.Context, c *ClientSideConnection, req ListSessionsRequest) iter.Seq2[SessionInfo, error] {
	return func(yield func(SessionInfo, error) bool) {
		if c == nil {
			yield(SessionInfo{}, errNilConnection)
			return
		}
		seen := map[string]bool{}
		if req.Cursor != nil {
			seen[*req.Cursor] = true
		}
		for {
			resp, err := c.ListSessions(ctx, req)
			if err != nil {
				yield(SessionInfo{}, err)
				return
			}
			for _, s := range resp.Sessions {
				if !yield(s, nil) {
					return
				}
			}
			if resp.NextCursor == nil || *resp.NextCursor == "" {
				return
			}
			next := *resp.NextCursor
			if seen[next] {
				yield(SessionInfo{}, ErrRepeatedCursor)
				return
			}
			seen[next] = true
			req.Cursor = &next
		}
	}
}

// Updated returns r.UpdatedAt as a time, read as RFC 3339, fractional
// seconds included; false when it is absent or not a date-time
// (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
func (r SessionInfo) Updated() (time.Time, bool) {
	if r.UpdatedAt == nil {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, *r.UpdatedAt)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// PickPermissionOption returns the first option whose kind is the first of
// kinds present in opts, trying each kind in turn. It reads only Kind, never
// Name, since PermissionOptionKind is a closed enum and a name such as
// "Don't allow" says nothing about the kind. It returns false when no option
// has any of kinds, or kinds is empty
// (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
func PickPermissionOption(opts []PermissionOption, kinds ...PermissionOptionKind) (PermissionOption, bool) {
	for _, k := range kinds {
		for _, o := range opts {
			if o.Kind == k {
				return o, true
			}
		}
	}
	return PermissionOption{}, false
}

// Chosen returns the option the client selected, and false when the
// outcome is cancelled, unknown or unset
// (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
func (r RequestPermissionResponse) Chosen() (PermissionOptionId, bool) {
	if r.Outcome.Selected == nil {
		return "", false
	}
	return r.Outcome.Selected.OptionId, true
}

// MetaObject returns meta[key] when it is a JSON object, and nil when it is
// absent or anything else. It is the read for a namespaced _meta entry, such
// as a vendor's capabilities (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
func MetaObject(meta map[string]any, key string) map[string]any {
	if m, ok := meta[key].(map[string]any); ok {
		return m
	}
	return nil
}

// DecodeMeta decodes meta[key] into a T through encoding/json. It returns
// false, and no error, when the key is absent, and true with the decode's
// error when the value does not fit T
// (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
func DecodeMeta[T any](meta map[string]any, key string) (T, bool, error) {
	var v T
	raw, ok := meta[key]
	if !ok {
		return v, false, nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return v, true, err
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return v, true, err
	}
	return v, true, nil
}

// ConfigOptionByCategory returns the first select option of opts whose
// category is c, such as SessionConfigOptionCategoryModel, and false when
// there is none (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
func ConfigOptionByCategory(opts []SessionConfigOption, c SessionConfigOptionCategory) (*SessionConfigOptionSelect, bool) {
	for _, o := range opts {
		if o.Select != nil && o.Select.Category != nil && *o.Select.Category == c {
			return o.Select, true
		}
	}
	return nil, false
}

// All returns every value o offers: Ungrouped as it is, or each group's
// options in order, so a client that lists values does not lose grouped
// ones. It returns nil when neither is set
// (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
func (o SessionConfigSelectOptions) All() []SessionConfigSelectOption {
	if o.Ungrouped != nil {
		return []SessionConfigSelectOption(*o.Ungrouped)
	}
	if o.Grouped == nil {
		return nil
	}
	var out []SessionConfigSelectOption
	for _, g := range *o.Grouped {
		out = append(out, g.Options...)
	}
	return out
}

// ResolveStdioCommands returns a WithInboundRewrite hook that resolves a
// stdio MCP server's command with lookPath, typically exec.LookPath, before
// the SDK validates it, since the protocol requires the command to be
// absolute and a client may send a bare name such as "npx". It reads the
// servers of session/new, session/load, session/resume and the unstable
// session/fork. A command filepath.IsAbs accepts is left as it is; one
// lookPath answers with an absolute path is replaced by it; any other,
// including a lookPath error, is left for the SDK to refuse -32602. The hook
// never refuses a message itself. Combine it with other rewrites through
// ChainRewrites (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
func ResolveStdioCommands(lookPath func(string) (string, error)) func(ctx context.Context, method string, params any) error {
	resolve := func(s *McpServerStdio) {
		if s == nil || filepath.IsAbs(s.Command) {
			return
		}
		if path, err := lookPath(s.Command); err == nil && filepath.IsAbs(path) {
			s.Command = path
		}
	}
	return func(_ context.Context, _ string, params any) error {
		switch p := params.(type) {
		case *NewSessionRequest:
			for _, s := range p.McpServers {
				resolve(s.Stdio)
			}
		case *LoadSessionRequest:
			for _, s := range p.McpServers {
				resolve(s.Stdio)
			}
		case *ResumeSessionRequest:
			for _, s := range p.McpServers {
				resolve(s.Stdio)
			}
		case *UnstableForkSessionRequest:
			for _, s := range p.McpServers {
				resolve(s.Stdio)
			}
		}
		return nil
	}
}

// OfferedAuthMethods returns methods, in order, without its terminal auth
// methods unless caps advertises auth.terminal, so an initialize answer built
// from it is one the SDK sends to that client. Agent and unknown methods are
// kept. It returns a new slice and leaves methods as it was
// (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
func OfferedAuthMethods(caps ClientCapabilities, methods []AuthMethod) []AuthMethod {
	out := make([]AuthMethod, 0, len(methods))
	for _, m := range methods {
		if m.Terminal != nil && !caps.Auth.Terminal {
			continue
		}
		out = append(out, m)
	}
	return out
}

// ToolCallStartOpt modifies a tool_call start update before it is built.
// Options apply in the order given (0024-PLAN-documentation.md, A10).
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
// Options apply in the order given (0024-PLAN-documentation.md, A10).
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
