package acp

import (
	"bytes"
	"encoding/json"
	"slices"
	"sync"
)

// ToolCallEntity is a tool call as a client has received it: the fields of
// its tool_call, as each tool_call_update has changed them
// (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 9).
type ToolCallEntity struct {
	Id        ToolCallId //nolint:revive // var-naming: spelled as CompactionEntity.Id is.
	Title     string
	Kind      ToolKind
	Status    ToolCallStatus
	Name      *string
	Content   []ToolCallContent
	Locations []ToolCallLocation
	RawInput  json.RawMessage
	RawOutput json.RawMessage
}

// ToolCallTimeline keeps the tool calls of one session as the protocol
// defines their updates: a client feeds it every SessionUpdate of the
// session, live or replayed by session/load, and reads the entities back. It
// holds no connection and sends nothing. The zero ToolCallTimeline is ready;
// it is safe for concurrent use. Like CompactionTimeline, it keeps one entity
// type whose rules the schema states, and is not a reducer of a session
// (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 9).
type ToolCallTimeline struct {
	mu       sync.Mutex
	entities map[ToolCallId]*ToolCallEntity
	order    []ToolCallId
}

// Apply applies u and returns the tool call it changed, as a copy, and true;
// for an update that is not a tool call's, it returns the zero entity and
// false.
//
//   - A tool_call is the call: it replaces every field of an entity of its
//     id, one already seen included, as a tool_call announces the whole call.
//   - A tool_call_update changes the fields it carries and leaves the rest,
//     as the schema says: "only changed fields need to be included". Its
//     content and locations replace the collections, an empty one clearing
//     them, as "Replace the content collection" says; an absent or null one
//     leaves them. A null name leaves the name, as the schema says, and a
//     null rawInput or rawOutput leaves it too, as a null raw value is an
//     absent one in this SDK
//     (0012-MADR-schema-fidelity-framework.md, "Amendment 2026-10-11, a null
//     raw value is an absent one").
//   - A tool_call_update for an id never seen starts the call with status
//     pending and no other field, then applies the update.
//
// Apply is the same for a live update and for one session/load replays.
func (t *ToolCallTimeline) Apply(u SessionUpdate) (ToolCallEntity, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	switch {
	case u.ToolCall != nil:
		c := u.ToolCall
		e := t.entity(c.ToolCallId)
		*e = ToolCallEntity{
			Id:        c.ToolCallId,
			Title:     c.Title,
			Kind:      c.Kind,
			Status:    c.Status,
			Name:      clonePtr(c.Name),
			Content:   slices.Clone(c.Content),
			Locations: slices.Clone(c.Locations),
			RawInput:  rawValue(c.RawInput),
			RawOutput: rawValue(c.RawOutput),
		}
		return copyToolCall(e), true
	case u.ToolCallUpdate != nil:
		c := u.ToolCallUpdate
		e := t.entity(c.ToolCallId)
		if c.Title != nil {
			e.Title = *c.Title
		}
		if c.Kind != nil {
			e.Kind = *c.Kind
		}
		if c.Status != nil {
			e.Status = *c.Status
		}
		if c.Name != nil {
			e.Name = clonePtr(c.Name)
		}
		if c.Content != nil {
			e.Content = slices.Clone(c.Content)
		}
		if c.Locations != nil {
			e.Locations = slices.Clone(c.Locations)
		}
		if raw := rawValue(c.RawInput); raw != nil {
			e.RawInput = raw
		}
		if raw := rawValue(c.RawOutput); raw != nil {
			e.RawOutput = raw
		}
		return copyToolCall(e), true
	default:
		return ToolCallEntity{}, false
	}
}

// Get returns the tool call id, as a copy, and whether the timeline has it.
func (t *ToolCallTimeline) Get(id ToolCallId) (ToolCallEntity, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.entities[id]
	if !ok {
		return ToolCallEntity{}, false
	}
	return copyToolCall(e), true
}

// Open returns the ids of the tool calls not known to have ended, of any
// status but completed and failed, one the protocol does not define
// included, in the order they were first seen: a client shows them as
// unfinished when a turn ends without a final status for them.
func (t *ToolCallTimeline) Open() []ToolCallId {
	t.mu.Lock()
	defer t.mu.Unlock()
	var open []ToolCallId
	for _, id := range t.order {
		switch t.entities[id].Status {
		case ToolCallStatusCompleted, ToolCallStatusFailed:
		default:
			open = append(open, id)
		}
	}
	return open
}

// entity returns the tool call id, starting it pending when it is first
// seen.
func (t *ToolCallTimeline) entity(id ToolCallId) *ToolCallEntity {
	if e, ok := t.entities[id]; ok {
		return e
	}
	if t.entities == nil {
		t.entities = map[ToolCallId]*ToolCallEntity{}
	}
	e := &ToolCallEntity{Id: id, Status: ToolCallStatusPending}
	t.entities[id] = e
	t.order = append(t.order, id)
	return e
}

// rawValue is a copy of raw, or nil for an absent or null one.
func rawValue(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	return bytes.Clone(raw)
}

// clonePtr is a pointer to a copy of *p, or nil.
func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// copyToolCall is a copy of e whose collections and raw values e does not
// share.
func copyToolCall(e *ToolCallEntity) ToolCallEntity {
	c := *e
	c.Name = clonePtr(e.Name)
	c.Content = slices.Clone(e.Content)
	c.Locations = slices.Clone(e.Locations)
	c.RawInput = bytes.Clone(e.RawInput)
	c.RawOutput = bytes.Clone(e.RawOutput)
	return c
}
