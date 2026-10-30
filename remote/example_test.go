package remote_test

import (
	"bufio"
	"context"
	"crypto/subtle"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/remote"
)

// agent answers the client; a real one implements every acp.Agent method.
type agent struct{ acp.Agent }

// client answers the agent; a real one implements every acp.Client method.
type client struct{ acp.Client }

// An agent served over the network, both profiles at one endpoint: the
// handler mounted on the host's own server, acp.ServeAgent each connection's
// serve, and the handler closed before the server shuts down.
func ExampleNewHandler() {
	h := remote.NewHandler(func(ctx context.Context, in io.Reader, out io.Writer) error {
		return acp.ServeAgent(ctx, agent{}, in, out)
	}, nil)
	mux := http.NewServeMux()
	// Authenticate every request here, not only initialize: the connection's
	// id is not a credential.
	mux.Handle("/acp", h)
	srv := &http.Server{Addr: "127.0.0.1:7331", Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()
	// Close first: Shutdown waits for every open connection, and an open
	// event stream is one until the handler ends it.
	bound, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := h.Close(bound); err != nil {
		log.Printf("handler: %v", err)
	}
	if err := srv.Shutdown(bound); err != nil {
		log.Printf("server: %v", err)
	}
}

// A client connected to a remote agent: Streamable HTTP for an https:// URL,
// WebSocket for a wss:// one, joined to the client as a child's pipes are.
func ExampleDial() {
	ctx := context.Background()
	c, err := remote.Dial(ctx, "https://agent.example/acp", &remote.DialOptions{
		Header: http.Header{"Authorization": {"Bearer " + os.Getenv("AGENT_TOKEN")}},
	})
	if err != nil {
		log.Fatal(err)
	}
	conn := acp.NewClientSideConnection(client{}, c, c)
	defer func() {
		if err := c.Close(); err != nil {
			log.Printf("close: %v", err)
		}
		<-conn.Done()
	}()
	if _, err := conn.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		log.Printf("initialize: %v", err)
	}
}

// A client's own HTTPClient, with its TLS settings and no Timeout, which
// would end the long-lived event streams; a header on every request; and a
// smaller message limit.
func ExampleDialOptions() {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS13}
	opts := &remote.DialOptions{
		HTTPClient:     &http.Client{Transport: transport},
		Header:         http.Header{"Authorization": {"Bearer " + os.Getenv("AGENT_TOKEN")}},
		MaxMessageSize: 1 << 20,
	}
	c, err := remote.Dial(context.Background(), "https://agent.example/acp", opts)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			log.Printf("close: %v", err)
		}
	}()
	conn := acp.NewClientSideConnection(client{}, c, c)
	if _, err := conn.Initialize(context.Background(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		log.Printf("initialize: %v", err)
	}
}

// A Handler behind the host's authentication, checked on every request, not
// only initialize: the connection's id is not a credential.
func ExampleHandler() {
	h := remote.NewHandler(func(ctx context.Context, in io.Reader, out io.Writer) error {
		return acp.ServeAgent(ctx, agent{}, in, out)
	}, nil)
	want := []byte("Bearer " + os.Getenv("AGENT_TOKEN"))
	http.Handle("/acp", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	}))
}

// A Conn read and written directly: one JSON-RPC message a line, each way,
// the first one initialize. After Close, Read returns io.EOF.
func ExampleConn() {
	c, err := remote.Dial(context.Background(), "wss://agent.example/acp", nil)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := io.WriteString(c, `{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":1}}`+"\n"); err != nil {
		log.Fatal(err)
	}
	answer, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(answer)
	if err := c.Close(); err != nil {
		log.Printf("close: %v", err)
	}
}

// A handler with a host's settings, whose serve logs why each connection
// ended. The ending cancels the serve's context with its cause, whether the
// serve is still running or has returned at its input's end.
func ExampleHandlerOptions() {
	opts := &remote.HandlerOptions{
		Grace:          2 * time.Second,
		KeepAlive:      10 * time.Second,
		MaxMessageSize: 4 << 20,
		MaxBuffered:    16 << 20,
		OriginPatterns: []string{"app.example.com"},
		Logger:         slog.Default(),
	}
	h := remote.NewHandler(func(ctx context.Context, in io.Reader, out io.Writer) error {
		context.AfterFunc(ctx, func() {
			switch cause := context.Cause(ctx); {
			case errors.Is(cause, remote.ErrDeleted):
				log.Print("the client ended the connection")
			case errors.Is(cause, remote.ErrDisconnected):
				log.Print("the client went away")
			case errors.Is(cause, remote.ErrOverflow):
				log.Print("the client did not read what it was sent")
			case errors.Is(cause, remote.ErrHandlerClosed):
				log.Print("the server is shutting down")
			}
		})
		return acp.ServeAgent(ctx, agent{}, in, out)
	}, opts)
	http.Handle("/acp", h)
}

// Closing a handler under a bound: every connection ends, and Close tells a
// serve it had to abandon from a bound that passed first.
func ExampleHandler_Close() {
	h := remote.NewHandler(func(ctx context.Context, in io.Reader, out io.Writer) error {
		return acp.ServeAgent(ctx, agent{}, in, out)
	}, nil)
	// The host serves h until it shuts down, and then:
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	switch err := h.Close(ctx); {
	case errors.Is(err, remote.ErrAbandoned):
		log.Printf("a serve ignored its context and was left running: %v", err)
	case errors.Is(err, context.DeadlineExceeded):
		log.Print("connections were still ending when the bound passed")
	case err != nil:
		log.Print(err)
	}
}
