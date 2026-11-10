package acp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/maccavelli/acp-go-sdk2/internal/tailbuf"
)

const (
	// defaultToolCallStreamInterval is the least time between two of a
	// stream's snapshots unless WithToolCallStreamInterval sets another.
	defaultToolCallStreamInterval = 250 * time.Millisecond
	// defaultToolCallStreamMaxText is the most live text a stream keeps
	// unless WithToolCallStreamMaxText sets another.
	defaultToolCallStreamMaxText = 1 << 20
)

// ToolCallStream sends one tool call of an agent as it runs: the call's
// whole state, as successive tool_call_update notifications, at most one per
// interval, the latest state each time. ACP v1 has no update that appends:
// each tool_call_update replaces the content and locations it carries
// ("Replace the content collection"), so a stream sends the whole state,
// which a client may drop or merge without losing any of it, and which
// ToolCallTimeline applies (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md).
//
// A stream is an io.Writer: what is written is the call's live text, which
// each snapshot carries as one text content item after the call's other
// content, its last 1 MiB by default. Writes, SetStatus and Update change the
// state and return at once; a sender, counted by the connection, writes it.
// Finish writes the final status. Every method is safe for concurrent use,
// but a change made while Finish runs may not be sent.
//
// A stream started on the context of an agent's Prompt belongs to that
// prompt turn: when Prompt returns, its unfinished streams are written before
// the turn is answered.
type ToolCallStream struct {
	conn    *AgentSideConnection
	session SessionId
	// base is the start's context without its cancellation: the background
	// writes keep its values, for middleware and the turn, and end only with
	// the connection.
	base     context.Context
	interval time.Duration
	// slot is held while a snapshot is taken and written, so the writes of
	// one stream are made in the order they were taken. A select on it can
	// end with a context, which a sync.Mutex cannot.
	slot chan struct{}

	mu       sync.Mutex
	entity   ToolCallEntity
	text     tailbuf.Buffer
	pending  bool      // a change not yet written
	running  bool      // a sender runs
	finished bool      // Finish has written the final update
	last     time.Time // when the last write ended
	err      error     // the first background write's error
}

// ToolCallStreamOption configures StartToolCallStream.
type ToolCallStreamOption func(*toolCallStreamConfig)

type toolCallStreamConfig struct {
	interval time.Duration
	maxText  int
}

// WithToolCallStreamInterval sets the least time between two of the
// stream's snapshots; d <= 0 keeps the default, 250 ms
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md,
// D2).
func WithToolCallStreamInterval(d time.Duration) ToolCallStreamOption {
	return func(c *toolCallStreamConfig) {
		if d > 0 {
			c.interval = d
		}
	}
}

// WithToolCallStreamMaxText sets the most bytes of live text the stream
// keeps, its last ones, cut at a character boundary; n <= 0 keeps the
// default, 1 MiB
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md,
// D4).
func WithToolCallStreamMaxText(n int) ToolCallStreamOption {
	return func(c *toolCallStreamConfig) {
		if n > 0 {
			c.maxText = n
		}
	}
}

// StartToolCallStream sends start, which must be a tool_call update such as
// StartToolCall returns, to session on conn, and returns the stream of that
// call, whose state start's fields begin. It returns SessionUpdate's error,
// such as ErrNotInitialized or a *CapabilityError, and a *ValidationError for
// a start that is not a tool_call; nothing is sent then. A stream started on
// the context of the agent's Prompt belongs to that turn
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md,
// D1).
func StartToolCallStream(ctx context.Context, conn *AgentSideConnection, session SessionId, start SessionUpdate,
	opts ...ToolCallStreamOption) (*ToolCallStream, error) {
	if start.ToolCall == nil {
		return nil, &ValidationError{Type: "SessionUpdate", Rule: RuleOneVariant,
			msg: "StartToolCallStream: start must be a tool_call update, as StartToolCall returns"}
	}
	cfg := toolCallStreamConfig{interval: defaultToolCallStreamInterval, maxText: defaultToolCallStreamMaxText}
	for _, opt := range opts {
		opt(&cfg)
	}
	if err := conn.SessionUpdate(ctx, SessionNotification{SessionId: session, Update: start}); err != nil {
		return nil, err
	}
	var started ToolCallTimeline
	entity, _ := started.Apply(start)
	s := &ToolCallStream{
		conn:     conn,
		session:  session,
		base:     context.WithoutCancel(ctx),
		interval: cfg.interval,
		slot:     make(chan struct{}, 1),
		entity:   entity,
		last:     time.Now(),
	}
	s.text.Limit = cfg.maxText
	return s, nil
}

// Write appends p to the call's live text. It never blocks on the wire, and
// returns len(p), also when the bound drops older text; after Finish it
// returns 0 and ErrToolCallStreamFinished.
func (s *ToolCallStream) Write(p []byte) (int, error) {
	if err := s.change(func() (bool, error) {
		s.text.Append(p)
		return len(p) > 0, nil
	}); err != nil {
		return 0, err
	}
	return len(p), nil
}

// WriteString is Write for a string.
func (s *ToolCallStream) WriteString(text string) (int, error) {
	if err := s.change(func() (bool, error) {
		s.text.AppendString(text)
		return text != "", nil
	}); err != nil {
		return 0, err
	}
	return len(text), nil
}

// SetStatus sets the call's status to pending or in_progress. A final status
// is Finish's: SetStatus refuses it, and any other, with an error wrapping
// ErrToolCallStreamField.
func (s *ToolCallStream) SetStatus(status ToolCallStatus) error {
	if status != ToolCallStatusPending && status != ToolCallStatusInProgress {
		return fmt.Errorf("acp: ToolCallStream.SetStatus(%q): %w", status, ErrToolCallStreamField)
	}
	return s.change(func() (bool, error) {
		if s.entity.Status == status {
			return false, nil
		}
		s.entity.Status = status
		return true, nil
	})
}

// Update runs f on the call's state under the stream's lock, for any change
// but the live text's and the status's: its title, kind, name, content in the
// order the agent chooses, locations, and rawInput and rawOutput as whole
// values. f must return quickly and must not call the stream. The call's Id
// and Status are the stream's: a change to either is undone and Update
// returns an error wrapping ErrToolCallStreamField that names the field; the
// other changes stand.
func (s *ToolCallStream) Update(f func(*ToolCallEntity)) error {
	return s.change(func() (bool, error) {
		id, status := s.entity.Id, s.entity.Status
		f(&s.entity)
		var field string
		switch {
		case s.entity.Id != id:
			field = "Id"
		case s.entity.Status != status:
			field = "Status"
		}
		s.entity.Id, s.entity.Status = id, status
		if field != "" {
			return true, fmt.Errorf("acp: ToolCallStream.Update changed %s: %w", field, ErrToolCallStreamField)
		}
		return true, nil
	})
}

// Entity returns the call's state as the stream holds it, the live text as
// its last content item: a copy that shares nothing with the stream.
func (s *ToolCallStream) Entity() ToolCallEntity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.entityLocked(s.finished)
}

// Dropped returns how many bytes of live text the bound has dropped from its
// front so far.
func (s *ToolCallStream) Dropped() int64 {
	return s.text.Dropped()
}

// Err returns the first error a background write met, such as a
// *ValidationError for a location that is not absolute, or
// ErrConnectionClosed once the connection has ended. The stream goes on with
// later changes, which may correct a refused value.
func (s *ToolCallStream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Flush writes the state now, after a write in flight, and returns the
// write's error; with nothing pending it returns once a write in flight is
// done. ctx is honoured but for a cancellation of the prompt turn's own:
// one whose cause wraps ErrPromptCancelled or ErrShutdown, after which
// writing the call's state is what the protocol asks. A Flush stopped before
// its write leaves the state for the sender.
func (s *ToolCallStream) Flush(ctx context.Context) error {
	wctx, cancel := wrapUp(ctx)
	defer cancel()
	return s.writePending(wctx)
}

// Finish sets the call's final status, completed or failed, and writes one
// update with it and the whole state, the live text whole, and returns the
// write's error. On success the stream is finished: later calls return
// ErrToolCallStreamFinished. On an error the status is restored and the
// stream is not finished, so Finish may be called again. ctx is honoured as
// Flush honours it, so a call cut by session/cancel still reaches its status
// on the context of the agent's Prompt
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md,
// D1).
func (s *ToolCallStream) Finish(ctx context.Context, status ToolCallStatus) error {
	if status != ToolCallStatusCompleted && status != ToolCallStatusFailed {
		return fmt.Errorf("acp: ToolCallStream.Finish(%q): %w", status, ErrToolCallStreamField)
	}
	wctx, cancel := wrapUp(ctx)
	defer cancel()
	if err := s.takeSlot(wctx); err != nil {
		return err
	}
	defer s.releaseSlot()
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return ErrToolCallStreamFinished
	}
	prev := s.entity.Status
	s.entity.Status = status
	u := toolCallUpdateOf(s.entityLocked(true))
	s.mu.Unlock()
	err := s.conn.SessionUpdate(wctx, SessionNotification{SessionId: s.session, Update: u})
	s.mu.Lock()
	defer s.mu.Unlock()
	s.last = time.Now()
	if err != nil {
		s.entity.Status = prev
		s.pending = true
		s.startLocked()
		return err
	}
	s.finished, s.pending = true, false
	return nil
}

// change runs f under the stream's lock and, when f reports a change, marks
// the state pending and starts a sender if none runs. After Finish it
// returns ErrToolCallStreamFinished and runs nothing.
func (s *ToolCallStream) change(f func() (bool, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return ErrToolCallStreamFinished
	}
	changed, err := f()
	if changed {
		s.pending = true
		s.startLocked()
	}
	return err
}

// startLocked starts the sender when none runs, through the connection's
// goIfLive, so the connection's wait group counts it; once the connection
// has ended, it keeps ErrConnectionClosed for Err. The caller holds mu.
func (s *ToolCallStream) startLocked() {
	if s.running || s.finished {
		return
	}
	s.running = true
	if !s.conn.Connection().goIfLive(s.run) {
		s.running = false
		s.keepErrLocked(ErrConnectionClosed)
	}
}

// run is the sender: while the state is pending, it waits until one interval
// has passed since the last write, then writes the state. It returns when it
// finds nothing pending, the stream finished, or the connection ended; it
// checks pending and clears running under one lock, so a change either is
// seen here or starts another sender.
func (s *ToolCallStream) run() {
	for {
		s.mu.Lock()
		if !s.pending || s.finished {
			s.running = false
			s.mu.Unlock()
			return
		}
		wait := s.interval - time.Since(s.last)
		s.mu.Unlock()
		if wait > 0 && !s.sleep(wait) {
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
			return
		}
		if err := s.writePending(s.base); err != nil && !errors.Is(err, ErrToolCallStreamFinished) {
			s.mu.Lock()
			s.keepErrLocked(err)
			s.mu.Unlock()
		}
	}
}

// sleep waits d, and reports false when the connection ends first.
func (s *ToolCallStream) sleep(d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-s.conn.Connection().Done():
		return false
	}
}

// writePending takes the slot, then, when the state is pending, takes its
// snapshot and writes it on ctx. A write stopped by ctx leaves the state
// pending and the sender started; a background write's ctx never ends, so a
// refused state is not written again until it changes.
func (s *ToolCallStream) writePending(ctx context.Context) error {
	if err := s.takeSlot(ctx); err != nil {
		return err
	}
	defer s.releaseSlot()
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return ErrToolCallStreamFinished
	}
	if !s.pending {
		s.mu.Unlock()
		return nil
	}
	u := toolCallUpdateOf(s.entityLocked(false))
	s.pending = false
	s.mu.Unlock()
	err := s.conn.SessionUpdate(ctx, SessionNotification{SessionId: s.session, Update: u})
	s.mu.Lock()
	defer s.mu.Unlock()
	s.last = time.Now()
	if err != nil && ctx.Err() != nil {
		s.pending = true
		s.startLocked()
	}
	return err
}

// takeSlot waits for the slot, or returns ctx's cause, or
// ErrConnectionClosed once the connection has ended.
func (s *ToolCallStream) takeSlot(ctx context.Context) error {
	select {
	case s.slot <- struct{}{}:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-s.conn.Connection().Done():
		return ErrConnectionClosed
	}
}

func (s *ToolCallStream) releaseSlot() { <-s.slot }

// keepErrLocked keeps err as Err's when none is kept. The caller holds mu.
func (s *ToolCallStream) keepErrLocked(err error) {
	if s.err == nil {
		s.err = err
	}
}

// entityLocked is a copy of the state, the live text as its last content
// item, the whole of it when final and without a character not yet finished
// otherwise. The caller holds mu.
func (s *ToolCallStream) entityLocked(final bool) ToolCallEntity {
	e := deepCopy(s.entity)
	if text, _ := s.text.Snapshot(final); text != "" {
		e.Content = append(e.Content, ToolContent(TextBlock(text)))
	}
	return e
}

// toolCallUpdateOf is the tool_call_update that carries e whole: every field
// it holds, its content and locations always, empty ones as [], which clear
// them (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md,
// D2). e is the caller's copy, whose fields the update may point to.
func toolCallUpdateOf(e ToolCallEntity) SessionUpdate {
	tu := SessionToolCallUpdate{
		ToolCallId: e.Id,
		Title:      &e.Title,
		Name:       e.Name,
		Content:    e.Content,
		Locations:  e.Locations,
		RawInput:   e.RawInput,
		RawOutput:  e.RawOutput,
	}
	if e.Kind != "" {
		tu.Kind = &e.Kind
	}
	if e.Status != "" {
		tu.Status = &e.Status
	}
	if tu.Content == nil {
		tu.Content = []ToolCallContent{}
	}
	if tu.Locations == nil {
		tu.Locations = []ToolCallLocation{}
	}
	return SessionUpdate{ToolCallUpdate: &tu}
}

// wrapUp returns a context that keeps ctx's values and ends when ctx ends,
// with ctx's cause, unless that cause wraps ErrPromptCancelled or
// ErrShutdown: the prompt turn's own end, after which the protocol asks for
// the calls' states to be written
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md,
// D1). The returned cancel releases it.
func wrapUp(ctx context.Context) (context.Context, context.CancelFunc) {
	out, cancel := context.WithCancelCause(context.WithoutCancel(ctx))
	end := func() {
		if cause := context.Cause(ctx); !errors.Is(cause, ErrPromptCancelled) && !errors.Is(cause, ErrShutdown) {
			cancel(cause)
		}
	}
	if ctx.Err() != nil {
		end()
		return out, func() { cancel(nil) }
	}
	stop := context.AfterFunc(ctx, end)
	return out, func() {
		stop()
		cancel(nil)
	}
}
