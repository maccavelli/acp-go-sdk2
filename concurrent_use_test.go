package acp

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// callTyped calls each typed method of side, a side connection, with ctx and
// zero params, and returns how many it called.
func callTyped(ctx context.Context, side any) int {
	n := 0
	v := reflect.ValueOf(side)
	for _, method := range v.Methods() {
		mt := method.Type()
		if mt.NumIn() != 2 || mt.In(0) != reflect.TypeFor[context.Context]() || mt.In(1).Kind() != reflect.Struct {
			continue
		}
		method.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.New(mt.In(1)).Elem()})
		n++
	}
	return n
}

// TestConcurrentUse calls the exported methods of Connection,
// AgentSideConnection and ClientSideConnection from several goroutines at
// once, over a pair connected by pipes: every send, each typed method among
// them, Connection, Done, SetLogger, Negotiated, DroppedNotifications, and at
// the end Close. Run under -race, it requires them safe for concurrent use.
// The notifications and requests it counts must all arrive or be answered,
// so the messages are written whole (0024-PLAN-documentation.md, A5).
func TestConcurrentUse(t *testing.T) {
	t.Parallel()
	const goroutines, rounds = 8, 25
	var cancels, updates, raws, typed atomic.Int64
	agent := agentFuncs{
		NewSessionFunc: func(context.Context, NewSessionRequest) (NewSessionResponse, error) {
			return NewSessionResponse{SessionId: "s"}, nil
		},
		CancelFunc: func(context.Context, CancelNotification) error { cancels.Add(1); return nil },
	}
	client := &clientFuncs{SessionUpdateFunc: func(context.Context, SessionNotification) error { updates.Add(1); return nil }}
	toAgent, fromClient := io.Pipe()
	toClient, fromAgent := io.Pipe()
	t.Cleanup(func() {
		_ = fromClient.Close()
		_ = fromAgent.Close()
	})
	asc := NewAgentSideConnection(agent, fromAgent, toAgent)
	csc := NewClientSideConnection(client, fromClient, toClient, WithInboundRewrite(func(_ context.Context, method string, _ any) error {
		if method == "_test/raw" {
			raws.Add(1)
		}
		return nil
	}))
	quiet := slog.New(slog.DiscardHandler)
	asc.SetLogger(quiet)
	csc.SetLogger(quiet)
	if _, err := csc.Initialize(t.Context(), InitializeRequest{ProtocolVersion: ProtocolVersionNumber}); err != nil {
		t.Fatal(err)
	}

	newSession := NewSessionRequest{Cwd: testAbsPath(), McpServers: []McpServer{}}
	update := SessionNotification{SessionId: "s", Update: UpdateAgentMessageText("hi")}
	errs := make(chan error, goroutines*rounds*4)
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range rounds {
				if _, err := csc.NewSession(t.Context(), newSession); err != nil {
					errs <- err
				}
				if err := csc.Cancel(t.Context(), CancelNotification{SessionId: "s"}); err != nil {
					errs <- err
				}
				if err := asc.SessionUpdate(t.Context(), update); err != nil {
					errs <- err
				}
				if err := asc.Connection().SendNotification(t.Context(), "_test/raw", map[string]any{}); err != nil {
					errs <- err
				}
				// Every other send, each typed method with zero params among
				// them: what each returns is not this test's to check.
				ctx, stop := context.WithTimeout(t.Context(), 5*time.Second)
				typed.Add(int64(callTyped(ctx, asc) + callTyped(ctx, csc)))
				for _, c := range []*Connection{asc.Connection(), csc.Connection()} {
					_ = c.SendRequestNoResult(ctx, "_test/request", map[string]any{})
					_, _ = SendRequest[json.RawMessage](c, ctx, "_test/request", map[string]any{})
				}
				_, _ = asc.CallExtension(ctx, "_test/call", map[string]any{})
				_, _ = csc.CallExtension(ctx, "_test/call", map[string]any{})
				_ = asc.NotifyExtension(ctx, "_test/notify", map[string]any{})
				_ = csc.NotifyExtension(ctx, "_test/notify", map[string]any{})
				stop()
				for _, c := range []*Connection{asc.Connection(), csc.Connection()} {
					c.SetLogger(quiet)
					_ = c.DroppedNotifications()
					select {
					case <-c.Done():
						errs <- ErrConnectionClosed
					default:
					}
				}
				asc.SetLogger(quiet)
				csc.SetLogger(quiet)
				_, _ = asc.Negotiated()
				_, _ = csc.Negotiated()
				_ = asc.DroppedNotifications()
				_ = csc.DroppedNotifications()
				_ = asc.Done()
				_ = csc.Done()
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if got := typed.Load(); got != goroutines*rounds*40 {
		t.Errorf("%d typed calls, want the 40 typed methods %d times", got, goroutines*rounds)
	}

	// Every notification sent arrives: a message written in pieces would not
	// decode, and would be lost.
	const want = goroutines * rounds
	deadline := time.Now().Add(10 * time.Second)
	for cancels.Load() < want || updates.Load() < want || raws.Load() < want {
		if time.Now().After(deadline) {
			t.Fatalf("received %d cancels, %d updates and %d raw notifications, want %d of each",
				cancels.Load(), updates.Load(), raws.Load(), want)
		}
		time.Sleep(10 * time.Millisecond)
	}

	var closing sync.WaitGroup
	for _, closer := range []func() error{asc.Close, csc.Close, asc.Connection().Close, csc.Connection().Close} {
		closing.Go(func() {
			if err := closer(); err != nil {
				t.Errorf("Close: %v", err)
			}
		})
	}
	closing.Wait()
	within(t, asc.Done(), "the agent's end")
	within(t, csc.Done(), "the client's end")
}
