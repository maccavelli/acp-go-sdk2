package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestConnectionInboundCancelRequest_CancelsHandler(t *testing.T) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	defer func() {
		_ = inW.Close()
		_ = outW.Close()
		_ = inR.Close()
		_ = outR.Close()
	}()

	started := make(chan struct{})
	c := NewConnection(func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
		close(started)
		<-ctx.Done()
		return nil, toReqErr(ctx.Err())
	}, outW, inR)
	_ = c

	lines := make(chan []byte, 10)
	go func() {
		scanner := bufio.NewScanner(outR)
		for scanner.Scan() {
			b := append([]byte(nil), scanner.Bytes()...)
			lines <- b
		}
		close(lines)
	}()

	// Send a request that will block until cancelled.
	_, err := inW.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"test","params":{}}` + "\n"))
	if err != nil {
		t.Fatalf("write request: %v", err)
	}

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start")
	}

	// Cancel the in-flight request.
	_, err = inW.Write([]byte(`{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":1}}` + "\n"))
	if err != nil {
		t.Fatalf("write cancel notification: %v", err)
	}

	var raw []byte
	select {
	case raw = <-lines:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for response")
	}

	var msg anyMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if msg.ID == nil {
		t.Fatalf("response missing id: %s", string(raw))
	}
	if got := string(*msg.ID); got != "1" {
		t.Fatalf("unexpected response id: %q", got)
	}
	if msg.Error == nil {
		t.Fatalf("expected error response, got: %s", string(raw))
	}
	if msg.Error.Code != -32800 {
		t.Fatalf("expected error code -32800, got %d (%s)", msg.Error.Code, msg.Error.Message)
	}
}

func TestConnectionInboundCancelRequest_CanonicalizesEquivalentIDs(t *testing.T) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	defer func() {
		_ = inW.Close()
		_ = outW.Close()
		_ = inR.Close()
		_ = outR.Close()
	}()

	started := make(chan struct{})
	c := NewConnection(func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
		close(started)
		<-ctx.Done()
		return nil, toReqErr(ctx.Err())
	}, outW, inR)
	_ = c

	lines := make(chan []byte, 10)
	go func() {
		scanner := bufio.NewScanner(outR)
		for scanner.Scan() {
			b := append([]byte(nil), scanner.Bytes()...)
			lines <- b
		}
		close(lines)
	}()

	// Request id is encoded as a unicode escape sequence; cancel uses the canonical form.
	_, err := inW.Write([]byte(`{"jsonrpc":"2.0","id":"\u0061","method":"test","params":{}}` + "\n"))
	if err != nil {
		t.Fatalf("write request: %v", err)
	}

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start")
	}

	_, err = inW.Write([]byte(`{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":"a"}}` + "\n"))
	if err != nil {
		t.Fatalf("write cancel notification: %v", err)
	}

	var raw []byte
	select {
	case raw = <-lines:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for response")
	}

	var msg anyMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if msg.Error == nil {
		t.Fatalf("expected error response, got: %s", string(raw))
	}
	if msg.Error.Code != -32800 {
		t.Fatalf("expected error code -32800, got %d (%s)", msg.Error.Code, msg.Error.Message)
	}
}

func TestConnectionInboundCancelRequest_CanonicalizesEquivalentNumericIDs(t *testing.T) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	defer func() {
		_ = inW.Close()
		_ = outW.Close()
		_ = inR.Close()
		_ = outR.Close()
	}()

	started := make(chan struct{})
	c := NewConnection(func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
		close(started)
		<-ctx.Done()
		return nil, toReqErr(ctx.Err())
	}, outW, inR)
	_ = c

	lines := make(chan []byte, 10)
	go func() {
		scanner := bufio.NewScanner(outR)
		for scanner.Scan() {
			b := append([]byte(nil), scanner.Bytes()...)
			lines <- b
		}
		close(lines)
	}()

	// Request id uses exponent notation; cancel uses normalized integer notation.
	_, err := inW.Write([]byte(`{"jsonrpc":"2.0","id":1e0,"method":"test","params":{}}` + "\n"))
	if err != nil {
		t.Fatalf("write request: %v", err)
	}

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start")
	}

	_, err = inW.Write([]byte(`{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":1}}` + "\n"))
	if err != nil {
		t.Fatalf("write cancel notification: %v", err)
	}

	var raw []byte
	select {
	case raw = <-lines:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for response")
	}

	var msg anyMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if msg.Error == nil {
		t.Fatalf("expected error response, got: %s", string(raw))
	}
	if msg.Error.Code != -32800 {
		t.Fatalf("expected error code -32800, got %d (%s)", msg.Error.Code, msg.Error.Message)
	}
}

func TestCanonicalJSONRPCIDKey_LargeNumericIDsDoNotCollide(t *testing.T) {
	id1 := json.RawMessage(`9007199254740992`)
	id2 := json.RawMessage(`9007199254740993`)

	key1, err := canonicalJSONRPCIDKey(id1)
	if err != nil {
		t.Fatalf("canonicalize id1: %v", err)
	}
	key2, err := canonicalJSONRPCIDKey(id2)
	if err != nil {
		t.Fatalf("canonicalize id2: %v", err)
	}

	if key1 != string(id1) {
		t.Fatalf("unexpected canonical id1: got %q want %q", key1, string(id1))
	}
	if key2 != string(id2) {
		t.Fatalf("unexpected canonical id2: got %q want %q", key2, string(id2))
	}
	if key1 == key2 {
		t.Fatalf("canonical keys collided: id1=%q id2=%q key=%q", id1, id2, key1)
	}
}

func TestCanonicalJSONRPCIDKey_NumericRepresentationsMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    json.RawMessage
		b    json.RawMessage
	}{
		{name: "integer exponent", a: json.RawMessage(`1`), b: json.RawMessage(`1e0`)},
		{name: "integer decimal", a: json.RawMessage(`1`), b: json.RawMessage(`1.0`)},
		{name: "fraction exponent", a: json.RawMessage(`0.1`), b: json.RawMessage(`1e-1`)},
		{name: "negative zero", a: json.RawMessage(`-0`), b: json.RawMessage(`0`)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			keyA, err := canonicalJSONRPCIDKey(tc.a)
			if err != nil {
				t.Fatalf("canonicalize a: %v", err)
			}
			keyB, err := canonicalJSONRPCIDKey(tc.b)
			if err != nil {
				t.Fatalf("canonicalize b: %v", err)
			}
			if keyA != keyB {
				t.Fatalf("expected equivalent numeric ids to match: a=%q b=%q keyA=%q keyB=%q", tc.a, tc.b, keyA, keyB)
			}
		})
	}
}

func TestCanonicalJSONRPCIDKey_RejectsOversizedExponent(t *testing.T) {
	t.Parallel()

	tests := []json.RawMessage{
		json.RawMessage(`1e4097`),
		json.RawMessage(`1e-4097`),
	}

	for _, raw := range tests {
		t.Run(string(raw), func(t *testing.T) {
			_, err := canonicalJSONRPCIDKey(raw)
			if !errors.Is(err, errJSONRPCNumericIDTooLarge) {
				t.Fatalf("expected oversized numeric id error for %q, got %v", raw, err)
			}
		})
	}
}

func TestConnectionResponseID_CanonicalizesEquivalentNumericRepresentations(t *testing.T) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	defer func() {
		_ = inW.Close()
		_ = outW.Close()
		_ = inR.Close()
		_ = outR.Close()
	}()

	c := NewConnection(nil, outW, inR)

	responderErr := make(chan error, 1)
	go func() {
		br := bufio.NewReader(outR)
		if _, err := br.ReadBytes('\n'); err != nil {
			responderErr <- fmt.Errorf("read outbound request: %w", err)
			return
		}
		if _, err := inW.Write([]byte(`{"jsonrpc":"2.0","id":1e0,"result":{"ok":true}}` + "\n")); err != nil {
			responderErr <- fmt.Errorf("write response: %w", err)
			return
		}
		responderErr <- nil
	}()

	result, err := SendRequest[map[string]bool](c, context.Background(), "test/method", map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("SendRequest returned error: %v", err)
	}
	if !result["ok"] {
		t.Fatalf("unexpected response payload: %#v", result)
	}

	if err := <-responderErr; err != nil {
		t.Fatal(err)
	}
}

func TestConnectionInboundCancelRequest_ImmediateCancelNoRace(t *testing.T) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	defer func() {
		_ = inW.Close()
		_ = outW.Close()
		_ = inR.Close()
		_ = outR.Close()
	}()

	c := NewConnection(func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
		<-ctx.Done()
		return nil, toReqErr(ctx.Err())
	}, outW, inR)
	_ = c

	lines := make(chan []byte, 10)
	go func() {
		scanner := bufio.NewScanner(outR)
		for scanner.Scan() {
			b := append([]byte(nil), scanner.Bytes()...)
			lines <- b
		}
		close(lines)
	}()

	for i := 1; i <= 25; i++ {
		payload := fmt.Sprintf(
			`{"jsonrpc":"2.0","id":%d,"method":"test","params":{}}`+"\n"+
				`{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":%d}}`+"\n",
			i, i,
		)
		if _, err := inW.Write([]byte(payload)); err != nil {
			t.Fatalf("write request/cancel pair %d: %v", i, err)
		}

		var raw []byte
		select {
		case raw = <-lines:
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for response on iteration %d", i)
		}

		var msg anyMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("unmarshal response on iteration %d: %v", i, err)
		}
		if msg.ID == nil {
			t.Fatalf("response missing id on iteration %d: %s", i, string(raw))
		}
		if got := string(*msg.ID); got != fmt.Sprintf("%d", i) {
			t.Fatalf("unexpected response id on iteration %d: got %q", i, got)
		}
		if msg.Error == nil {
			t.Fatalf("expected error response on iteration %d, got: %s", i, string(raw))
		}
		if msg.Error.Code != -32800 {
			t.Fatalf("expected error code -32800 on iteration %d, got %d (%s)", i, msg.Error.Code, msg.Error.Message)
		}
	}
}

func TestConnectionOutboundCancelRequest_SendsNotification(t *testing.T) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	defer func() {
		_ = inW.Close()
		_ = outW.Close()
		_ = inR.Close()
		_ = outR.Close()
	}()

	c := NewConnection(nil, outW, inR)

	lines := make(chan []byte, 10)
	go func() {
		scanner := bufio.NewScanner(outR)
		for scanner.Scan() {
			b := append([]byte(nil), scanner.Bytes()...)
			lines <- b
		}
		close(lines)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := SendRequest[json.RawMessage](c, ctx, "test/method", map[string]any{"x": 1})
		errCh <- err
	}()

	// First message should be the outbound request.
	var reqRaw []byte
	select {
	case reqRaw = <-lines:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for request")
	}

	var req anyMessage
	if err := json.Unmarshal(reqRaw, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if req.ID == nil {
		t.Fatalf("request missing id: %s", string(reqRaw))
	}
	if req.Method != "test/method" {
		t.Fatalf("unexpected request method: %q", req.Method)
	}
	idKey := string(*req.ID)

	// Cancel the outbound request context; this should trigger a best-effort $/cancel_request.
	cancel()

	var cancelRaw []byte
	select {
	case cancelRaw = <-lines:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for cancel notification")
	}

	var cancelMsg anyMessage
	if err := json.Unmarshal(cancelRaw, &cancelMsg); err != nil {
		t.Fatalf("unmarshal cancel notification: %v", err)
	}
	if cancelMsg.ID != nil {
		t.Fatalf("cancel notification unexpectedly had id: %s", string(cancelRaw))
	}
	if cancelMsg.Method != "$/cancel_request" {
		t.Fatalf("unexpected cancel method: %q", cancelMsg.Method)
	}

	var p cancelRequestParams
	if err := json.Unmarshal(cancelMsg.Params, &p); err != nil {
		t.Fatalf("unmarshal cancel params: %v", err)
	}
	if got := string(p.RequestID); got != idKey {
		t.Fatalf("unexpected cancel requestId: got %q want %q", got, idKey)
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected request error")
		}
		var re *RequestError
		if !errors.As(err, &re) {
			t.Fatalf("expected *RequestError, got %T: %v", err, err)
		}
		if re.Code != -32800 {
			t.Fatalf("expected error code -32800, got %d", re.Code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SendRequest to return")
	}
}

func TestConnectionOutboundRequestTimeout_ReturnsInternalError(t *testing.T) {
	inR, inW := io.Pipe()
	defer func() {
		_ = inW.Close()
		_ = inR.Close()
	}()

	c := NewConnection(nil, io.Discard, inR)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := SendRequest[json.RawMessage](c, ctx, "test/method", map[string]any{"x": 1})
	if err == nil {
		t.Fatal("expected request error")
	}

	var re *RequestError
	if !errors.As(err, &re) {
		t.Fatalf("expected *RequestError, got %T: %v", err, err)
	}
	if re.Code != -32603 {
		t.Fatalf("expected timeout to map to internal error code -32603, got %d (%s)", re.Code, re.Message)
	}

	c.mu.Lock()
	pendingCount := len(c.pending)
	c.mu.Unlock()
	if pendingCount != 0 {
		t.Fatalf("expected pending map to be cleaned up after timeout, got %d entries", pendingCount)
	}
}

func TestConnectionOutboundCancelRequest_DoesNotBlockWhenPeerStopsReading(t *testing.T) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	defer func() {
		_ = inW.Close()
		_ = outW.Close()
		_ = inR.Close()
		_ = outR.Close()
	}()

	c := NewConnection(nil, outW, inR)

	firstReq := make(chan []byte, 1)
	go func() {
		br := bufio.NewReader(outR)
		line, err := br.ReadBytes('\n')
		if err == nil {
			firstReq <- append([]byte(nil), line...)
		}
		close(firstReq)
		// Intentionally stop reading after the first request line.
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := SendRequest[json.RawMessage](c, ctx, "test/method", map[string]any{"x": 1})
		errCh <- err
	}()

	var reqRaw []byte
	select {
	case reqRaw = <-firstReq:
		if len(reqRaw) == 0 {
			t.Fatal("failed to read first outbound request")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first request")
	}

	var req anyMessage
	if err := json.Unmarshal(reqRaw, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if req.ID == nil {
		t.Fatalf("request missing id: %s", string(reqRaw))
	}

	// Peer is no longer reading. The best-effort cancel write may block in the background,
	// but SendRequest should still return promptly on context cancellation.
	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected request error")
		}
		var re *RequestError
		if !errors.As(err, &re) {
			t.Fatalf("expected *RequestError, got %T: %v", err, err)
		}
		if re.Code != -32800 {
			t.Fatalf("expected error code -32800, got %d", re.Code)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("SendRequest blocked on cancel notification write")
	}
}

func TestConnectionSendCancelRequest_BoundsPendingQueue(t *testing.T) {
	baseCtx, baseCancel := context.WithCancelCause(context.Background())
	defer baseCancel(nil)

	c := &Connection{
		pending:             make(map[string]*pendingResponse),
		inflight:            make(map[string]*inflightRequest),
		cancelRequestSignal: make(chan struct{}, 1),
		ctx:                 baseCtx,
		cancel:              baseCancel,
	}

	for i := range maxPendingCancelRequests + 128 {
		c.sendCancelRequest(fmt.Sprintf("%d", i))
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.pendingCancelRequest) != maxPendingCancelRequests {
		t.Fatalf("expected pending cancel queue length %d, got %d", maxPendingCancelRequests, len(c.pendingCancelRequest))
	}

	if got := c.pendingCancelRequest[0]; got != "0" {
		t.Fatalf("expected queue to retain earliest id when full, got first id %q", got)
	}

	expectedLast := fmt.Sprintf("%d", maxPendingCancelRequests-1)
	if got := c.pendingCancelRequest[len(c.pendingCancelRequest)-1]; got != expectedLast {
		t.Fatalf("expected queue to drop ids beyond capacity, got last id %q want %q", got, expectedLast)
	}
}

func TestConnectionWaitForResponse_PeerDisconnectWinsOverDerivedContextCancel(t *testing.T) {
	const iterations = 64

	for i := range iterations {
		baseCtx, baseCancel := context.WithCancelCause(context.Background())
		c := &Connection{
			pending: make(map[string]*pendingResponse),
			ctx:     baseCtx,
			cancel:  baseCancel,
		}

		idKey := fmt.Sprintf("id-%d", i)
		pr := &pendingResponse{ch: make(chan responseEnvelope)}
		c.pending[idKey] = pr

		requestCtx, requestCancel := context.WithCancel(baseCtx)
		baseCancel(errors.New("peer closed"))

		_, err := c.waitForResponse(requestCtx, pr, idKey)
		requestCancel()

		if err == nil {
			t.Fatalf("iteration %d: expected error", i)
		}
		var re *RequestError
		if !errors.As(err, &re) {
			t.Fatalf("iteration %d: expected *RequestError, got %T (%v)", i, err, err)
		}
		if re.Code != -32603 {
			t.Fatalf("iteration %d: expected disconnect error code -32603, got %d (%s)", i, re.Code, re.Message)
		}

		if _, ok := c.pending[idKey]; ok {
			t.Fatalf("iteration %d: pending request %q was not cleaned up", i, idKey)
		}
	}
}

// TestRequestWithEndedContextWritesNothing requires a request whose context has
// already ended to return its end and write nothing, neither the request nor a
// $/cancel_request for it (0019-PLAN-behaviour-fixes.md, B9). It runs in a
// synctest bubble, so it counts once every goroutine is blocked
// (0019-PLAN-examples-docs-tests-tooling.md, E10).
func TestRequestWithEndedContextWritesNothing(t *testing.T) {
	t.Parallel()
	synctest.Test(t, testRequestWithEndedContextWritesNothing)
}

func testRequestWithEndedContextWritesNothing(t *testing.T) {
	w := &countingWriter{}
	c := NewConnection(nil, w, idleReader(t))

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	expired, stop := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer stop()
	for _, tt := range []struct {
		name string
		ctx  context.Context
		code int
	}{{"cancelled", cancelled, -32800}, {"past its deadline", expired, -32603}} {
		_, err := SendRequest[json.RawMessage](c, tt.ctx, "_test/x", map[string]any{})
		if re, ok := errors.AsType[*RequestError](err); !ok || re.Code != tt.code {
			t.Errorf("SendRequest, context %s: %v, want %d", tt.name, err, tt.code)
		}
		err = c.SendRequestNoResult(tt.ctx, "_test/x", map[string]any{})
		if re, ok := errors.AsType[*RequestError](err); !ok || re.Code != tt.code {
			t.Errorf("SendRequestNoResult, context %s: %v, want %d", tt.name, err, tt.code)
		}
	}
	// A cancel request would be written by its own goroutine: wait until
	// every goroutine is blocked.
	synctest.Wait()
	if n := w.count(); n != 0 {
		t.Fatalf("%d bytes written for requests whose context had ended", n)
	}
}

// TestPromptWithEndedContextWritesNothing requires Prompt, called with a
// context that has already ended, to write neither session/prompt nor a
// session/cancel for it: the agent got no turn to cancel
// (0019-PLAN-issue-fixes.md, B22).
func TestPromptWithEndedContextWritesNothing(t *testing.T) {
	t.Parallel()
	var csc *ClientSideConnection
	p := rawWire(t, func(w io.Writer, r io.Reader) {
		csc = NewClientSideConnection(&clientFuncs{}, w, r)
		negotiate(t, csc, allAdvertised())
	})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := csc.Prompt(ctx, PromptRequest{SessionId: "s1", Prompt: []ContentBlock{TextBlock("hi")}})
	if re, ok := errors.AsType[*RequestError](err); !ok || re.Code != -32800 {
		t.Fatalf("Prompt with an ended context = %v, want -32800", err)
	}
	// A marker sent after Prompt returned is the first line written.
	if err := csc.NotifyExtension(t.Context(), "_marker", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	var m struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal([]byte(within(t, p.lines, "the first line")), &m); err != nil {
		t.Fatal(err)
	}
	if m.Method != "_marker" {
		t.Errorf("Prompt with an ended context wrote %q, want nothing", m.Method)
	}
}

// TestPromptEndedBeforeHandOverSendsNoCancel requires Prompt whose context
// ends while its request waits for the writer, which a peer that stopped
// reading holds, to return without sending session/cancel: the request was
// never handed over, so nothing reached the agent. Before B22 Prompt sent the
// cancel and blocked on the same writer. It runs in a synctest bubble, so the
// context ends once the request waits (0019-PLAN-issue-fixes.md, B22).
func TestPromptEndedBeforeHandOverSendsNoCancel(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		outR, outW := io.Pipe() // never read: the first write blocks
		inR, inW := io.Pipe()
		t.Cleanup(func() {
			_ = outR.CloseWithError(errors.New("test over"))
			_ = inW.Close()
		})
		csc := NewClientSideConnection(&clientFuncs{}, outW, inR)
		negotiate(t, csc, allAdvertised())
		go func() { _ = csc.NotifyExtension(context.WithoutCancel(t.Context()), "_stall", map[string]any{}) }()
		synctest.Wait() // the writer is blocked on the unread pipe

		ctx, cancel := context.WithCancel(t.Context())
		returned := make(chan error, 1)
		go func() {
			_, err := csc.Prompt(ctx, PromptRequest{SessionId: "s1", Prompt: []ContentBlock{TextBlock("hi")}})
			returned <- err
		}()
		synctest.Wait() // Prompt waits for the writer to take its request
		cancel()
		err := <-returned
		if re, ok := errors.AsType[*RequestError](err); !ok || re.Code != -32800 {
			t.Errorf("Prompt = %v, want -32800", err)
		}
	})
}

// refusingWriter writes to w until refuse is set, then refuses every write.
type refusingWriter struct {
	w      io.Writer
	refuse atomic.Bool
}

func (r *refusingWriter) Write(p []byte) (int, error) {
	if r.refuse.Load() {
		return 0, errors.New("refused")
	}
	return r.w.Write(p)
}

// TestPromptLogsFailedCancel requires Prompt, whose context ends after its
// request was handed to the writer, to log at Debug the session/cancel it
// could not write, rather than drop its error
// (0019-PLAN-idioms-performance.md, deviation during step 3).
func TestPromptLogsFailedCancel(t *testing.T) {
	t.Parallel()
	var logs lockedBuffer
	debug := WithLogger(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	out := &refusingWriter{}
	var csc *ClientSideConnection
	p := rawWire(t, func(w io.Writer, r io.Reader) {
		out.w = w
		csc = NewClientSideConnection(&clientFuncs{}, out, r, debug)
		negotiate(t, csc, allAdvertised())
	})
	ctx, cancel := context.WithCancel(t.Context())
	returned := make(chan error, 1)
	go func() {
		_, err := csc.Prompt(ctx, PromptRequest{SessionId: "s1", Prompt: []ContentBlock{TextBlock("hi")}})
		returned <- err
	}()
	within(t, p.lines, "the session/prompt request")
	out.refuse.Store(true)
	cancel()
	err := within(t, returned, "Prompt's return")
	if re, ok := errors.AsType[*RequestError](err); !ok || re.Code != -32800 {
		t.Errorf("Prompt = %v, want -32800", err)
	}
	if got := logs.String(); !strings.Contains(got, "level=DEBUG msg=\"failed to send the cancel of a prompt turn\" method=session/cancel") {
		t.Errorf("no Debug record of the failed session/cancel; the log holds:\n%s", got)
	}
}

// TestDuplicateInflightIDKeepsLaterCancel requires a request that reuses the id
// of one still in flight to stay reachable by $/cancel_request after the first
// returns (0019-PLAN-behaviour-fixes.md, B11).
func TestDuplicateInflightIDKeepsLaterCancel(t *testing.T) {
	t.Parallel()
	releaseFirst := make(chan struct{})
	firstStarted := make(chan struct{})
	second := make(chan context.Context, 1)
	handler := func(ctx context.Context, method string, _ json.RawMessage) (any, *RequestError) {
		switch method {
		case "_test/first":
			close(firstStarted)
			<-releaseFirst
			return "one", nil
		case "_test/second":
			second <- ctx
			<-ctx.Done()
			return nil, toReqErr(ctx.Err())
		}
		return nil, NewMethodNotFound(method)
	}
	p := rawWire(t, func(w io.Writer, r io.Reader) { NewConnection(handler, w, r) })
	p.send(`{"jsonrpc":"2.0","id":7,"method":"_test/first"}`)
	within(t, firstStarted, "the first request's handler")
	p.send(`{"jsonrpc":"2.0","id":7,"method":"_test/second"}`)
	secondCtx := within(t, second, "the second request's handler")
	close(releaseFirst)
	if a := p.next("the first request's answer"); string(a.Result) != `"one"` {
		t.Fatalf("first answer: %+v", a)
	}
	p.send(`{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":7}}`)
	select {
	case <-secondCtx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("$/cancel_request did not reach the request that reused the id")
	}
	if a := p.next("the second request's answer"); a.Error == nil || a.Error.Code != -32800 {
		t.Fatalf("second answer: %+v, want -32800", a)
	}
}

// blockingRequests starts a connection, with opts, whose handler reports the
// params of each request it runs on started and returns when release yields.
func blockingRequests(t *testing.T, started chan<- string, release <-chan struct{}, opts ...ConnectionOption) wire {
	t.Helper()
	return rawWire(t, func(w io.Writer, r io.Reader) {
		NewConnection(func(_ context.Context, _ string, params json.RawMessage) (any, *RequestError) {
			started <- string(params)
			<-release
			return map[string]any{}, nil
		}, w, r, opts...)
	})
}

// TestMaxConcurrentRequests requires a request over the limit of
// WithMaxConcurrentRequests to be answered -32603 at once without its handler
// running, and a request sent once a running one has answered to run; with
// no limit, 200 blocking requests all run (0019-PLAN-api-decisions.md, B18).
func TestMaxConcurrentRequests(t *testing.T) {
	t.Parallel()
	t.Run("limit 2", func(t *testing.T) {
		t.Parallel()
		started, release := make(chan string, 8), make(chan struct{})
		p := blockingRequests(t, started, release, WithMaxConcurrentRequests(2))
		request := func(n int) {
			p.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"_x/r","params":{"n":%d}}`, n, n))
		}
		for n := 1; n <= 3; n++ {
			request(n)
		}
		a := p.next("the third request's answer")
		if string(a.ID) != "3" || a.Error == nil || a.Error.Code != -32603 {
			t.Fatalf("first answer: id %s, %+v; want id 3 refused -32603", a.ID, a.Error)
		}
		if data, _ := a.Error.Data.(map[string]any); data["error"] != "too many concurrent requests" || data["limit"] != float64(2) {
			t.Errorf("refusal data %v, want the error and limit 2", a.Error.Data)
		}

		ran := map[string]bool{}
		for range 2 {
			ran[within(t, started, "a request under the limit")] = true
		}
		release <- struct{}{}
		if a := p.next("a released request's answer"); a.Error != nil {
			t.Fatalf("a released request answered %+v", a.Error)
		}
		request(4)
		ran[within(t, started, "a request sent once one had answered")] = true
		close(release)
		for range 2 {
			p.next("the remaining answers")
		}
		for len(started) > 0 {
			ran[<-started] = true
		}
		if want := map[string]bool{`{"n":1}`: true, `{"n":2}`: true, `{"n":4}`: true}; !maps.Equal(ran, want) {
			t.Errorf("handlers ran for %v, want requests 1, 2 and 4", slices.Sorted(maps.Keys(ran)))
		}
	})
	t.Run("unlimited by default", func(t *testing.T) {
		t.Parallel()
		started, release := make(chan string, 200), make(chan struct{})
		p := blockingRequests(t, started, release)
		for n := 1; n <= 200; n++ {
			p.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"_x/r","params":{}}`, n))
		}
		for range 200 {
			within(t, started, "one of 200 blocking requests")
		}
		close(release)
	})
}
