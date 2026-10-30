package remote_test

import (
	"context"
	"errors"
	"io"
	"log"
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
