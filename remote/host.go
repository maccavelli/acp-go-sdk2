package remote

import (
	"net"
	"net/http"
	"net/netip"
	"path"
	"strings"
)

// hostAllowed applies HandlerOptions.AllowedHosts to a request's Host
// (0049-PLAN-subpackage-fixes.md, phase 4, P2): with patterns, the Host's
// name, its port dropped, must match one, as originAllowed matches; without,
// on a listener whose address is loopback, it must be an IP literal,
// localhost or a name under .localhost, since a DNS rebinding gives a page's
// requests its own name; on any other listener, or a request that does not
// say which, every name passes.
func hostAllowed(r *http.Request, patterns []string) bool {
	name := hostName(r.Host)
	if len(patterns) > 0 {
		for _, p := range patterns {
			if ok, err := path.Match(strings.ToLower(p), name); err == nil && ok {
				return true
			}
		}
		return false
	}
	local, _ := r.Context().Value(http.LocalAddrContextKey).(net.Addr) //nolint:errcheck // a missing address is a listener that says nothing, which checks nothing.
	tcp, ok := local.(*net.TCPAddr)
	if !ok || !tcp.IP.IsLoopback() {
		return true
	}
	if _, err := netip.ParseAddr(name); err == nil {
		return true
	}
	return name == "localhost" || strings.HasSuffix(name, ".localhost")
}

// hostName is host, a Host header, without its port, its IPv6 brackets or a
// final dot, lower-cased.
func hostName(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	return strings.ToLower(strings.TrimSuffix(host, "."))
}
