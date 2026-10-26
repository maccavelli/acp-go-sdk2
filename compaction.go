package acp

import (
	"maps"
	"slices"
	"sync"
)

// CompactionEntity is a compaction as a client has received it: its id, its
// status, as it arrived, an unknown one included, its summary, its error and
// its metadata. Seen is true once a compaction_update for the id has arrived;
// an entity known only from summary chunks has it false
// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 3).
type CompactionEntity struct {
	Id      CompactionId //nolint:revive // var-naming: spelled as the generated types spell it, CompactionUpdate.CompactionId among them.
	Status  CompactionStatus
	Summary []ContentBlock
	Error   string
	Meta    map[string]any
	Seen    bool
}

// CompactionTimeline keeps the compactions of one session as the protocol
// defines their updates: a client that advertises session.compaction feeds
// it every SessionUpdate of the session, live or replayed by session/load,
// and reads the entities back. It holds no connection and sends nothing.
// The zero CompactionTimeline is ready; it is safe for concurrent use
// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 3).
type CompactionTimeline struct {
	mu       sync.Mutex
	entities map[CompactionId]*CompactionEntity
	order    []CompactionId
}

// Apply applies u and returns the compaction it changed, as a copy, and
// true; for an update that is not a compaction's, it returns the zero
// entity and false.
//
//   - A compaction_update for an id first seen starts it in_progress with
//     no summary, error or metadata, then applies the update.
//   - A compaction_update applies each field that is a patch as the
//     protocol says: absent leaves it, null clears it, a value replaces it;
//     an empty summary clears it too. Its status replaces the status, an
//     unknown one kept as it arrived.
//   - A compaction_summary_chunk appends its content to the summary. A chunk
//     for an id first seen starts it as a compaction_update would, which the
//     protocol's first-seen rule implies without naming chunks.
//
// Apply is the same for a live update and for one session/load replays: an
// omitted field is never reset.
func (t *CompactionTimeline) Apply(u SessionUpdate) (CompactionEntity, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	switch {
	case u.CompactionUpdate != nil:
		c := u.CompactionUpdate
		e := t.entity(c.CompactionId)
		e.Seen = true
		e.Status = c.Status
		if c.Summary.IsNull() {
			e.Summary = nil
		} else if s, ok := c.Summary.Value(); ok {
			e.Summary = slices.Clone(s)
			if len(e.Summary) == 0 {
				e.Summary = nil
			}
		}
		if c.Error.IsNull() {
			e.Error = ""
		} else if s, ok := c.Error.Value(); ok {
			e.Error = s
		}
		if c.Meta.IsNull() {
			e.Meta = nil
		} else if m, ok := c.Meta.Value(); ok {
			e.Meta = maps.Clone(m)
		}
		return copyEntity(e), true
	case u.CompactionSummaryChunk != nil:
		c := u.CompactionSummaryChunk
		e := t.entity(c.CompactionId)
		e.Summary = append(e.Summary, c.Content)
		return copyEntity(e), true
	default:
		return CompactionEntity{}, false
	}
}

// Get returns the compaction id, as a copy, and whether the timeline has it.
func (t *CompactionTimeline) Get(id CompactionId) (CompactionEntity, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.entities[id]
	if !ok {
		return CompactionEntity{}, false
	}
	return copyEntity(e), true
}

// Open returns the ids of the compactions not known to have ended, in_progress
// or of a status the protocol does not define, in the order they were first
// seen: a client shows them as unfinished when a turn ends without a terminal
// status for them.
func (t *CompactionTimeline) Open() []CompactionId {
	t.mu.Lock()
	defer t.mu.Unlock()
	var open []CompactionId
	for _, id := range t.order {
		switch t.entities[id].Status {
		case CompactionStatusCompleted, CompactionStatusFailed, CompactionStatusCancelled:
		default:
			open = append(open, id)
		}
	}
	return open
}

// entity returns the compaction id, starting it in_progress when it is first
// seen.
func (t *CompactionTimeline) entity(id CompactionId) *CompactionEntity {
	if e, ok := t.entities[id]; ok {
		return e
	}
	if t.entities == nil {
		t.entities = map[CompactionId]*CompactionEntity{}
	}
	e := &CompactionEntity{Id: id, Status: CompactionStatusInProgress}
	t.entities[id] = e
	t.order = append(t.order, id)
	return e
}

// copyEntity is a copy of e whose summary and metadata e does not share.
func copyEntity(e *CompactionEntity) CompactionEntity {
	c := *e
	c.Summary = slices.Clone(e.Summary)
	c.Meta = maps.Clone(e.Meta)
	return c
}
