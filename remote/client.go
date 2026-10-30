package remote

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
)

// defaultMaxMessageSize is the largest message either side takes unless its
// options set another: the connection's own default, 10 MiB.
const defaultMaxMessageSize = 10 * 1024 * 1024

// errHTTPNotYet is Dial's error for an http:// or https:// URL until the
// Streamable HTTP client lands (0043-PLAN-remote-http-and-websocket-transport.md,
// phase 4).
var errHTTPNotYet = errors.New("remote: Streamable HTTP is not implemented yet")

// DialOptions configure Dial. The zero value, or nil, is ready to use.
type DialOptions struct {
	// HTTPClient makes the connection's requests: a WebSocket's handshake, or
	// Streamable HTTP's requests. Nil is a new http.Client. Its Jar, when nil,
	// is a new cookie jar of the connection's own, as the RFD requires a
	// client to keep the server's cookies. For WebSocket, an *http.Transport
	// is copied and limited to HTTP/1.1, which the handshake needs.
	HTTPClient *http.Client

	// Header is sent on every request the connection makes, the handshake
	// included: authentication, for one, which the RFD leaves to the host.
	Header http.Header

	// MaxMessageSize is the largest message, in bytes, the connection reads
	// or writes. Zero or less is 10 MiB, the connection's own default.
	MaxMessageSize int
}

// Conn is a client's remote connection to an agent: an io.ReadWriteCloser of
// newline-ended JSON-RPC messages, for acp.NewClientSideConnection's two
// streams.
type Conn struct {
	t transport
}

// transport is what a profile gives a Conn.
type transport interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
}

// Read reads the messages from the agent, one line each. It returns io.EOF
// after the agent's clean end, and the transport's error after any other.
func (c *Conn) Read(p []byte) (int, error) { return c.t.Read(p) }

// Write sends each line written as one message. A line longer than
// MaxMessageSize is not sent, and Write returns an error for it.
func (c *Conn) Write(p []byte) (int, error) { return c.t.Write(p) }

// Close ends the connection: a WebSocket's close handshake. Read then
// returns io.EOF.
func (c *Conn) Close() error { return c.t.Close() }

// Dial connects to the agent at rawURL. A ws:// or wss:// URL is the
// WebSocket profile, and ctx bounds its handshake; ctx does not bound the
// connection, which lasts until Close or its end. An http:// or https:// URL
// is the Streamable HTTP profile, which the RFD requires a client to speak
// too.
func Dial(ctx context.Context, rawURL string, opts *DialOptions) (*Conn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("remote: %w", err)
	}
	o := resolveDialOptions(opts)
	switch u.Scheme {
	case "ws", "wss":
		t, err := dialWebSocket(ctx, rawURL, o)
		if err != nil {
			return nil, err
		}
		return &Conn{t: t}, nil
	case "http", "https":
		return nil, errHTTPNotYet
	default:
		return nil, fmt.Errorf("remote: the URL's scheme is %q, not ws, wss, http or https", u.Scheme)
	}
}

// resolveDialOptions returns opts with its defaults set, and its client a
// copy, so the caller's is not changed.
func resolveDialOptions(opts *DialOptions) DialOptions {
	var o DialOptions
	if opts != nil {
		o = *opts
	}
	client := &http.Client{}
	if o.HTTPClient != nil {
		c := *o.HTTPClient
		client = &c
	}
	if client.Jar == nil {
		// cookiejar.New returns no error for nil options.
		client.Jar, _ = cookiejar.New(nil) //nolint:errcheck // cookiejar.New fails only on its options, which are nil.
	}
	o.HTTPClient = client
	if o.MaxMessageSize <= 0 {
		o.MaxMessageSize = defaultMaxMessageSize
	}
	return o
}
