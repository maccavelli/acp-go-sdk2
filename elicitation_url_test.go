package acp_test

import (
	"errors"
	"strings"
	"testing"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// TestParseElicitationURL refuses a URL a client must not offer to open, with
// a message that holds nothing of it, and reports the host to show, whether
// the scheme is https, and whether the host is Punycode.
func TestParseElicitationURL(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		"",
		"not a url",
		"/relative",
		"example.com",
		"javascript:alert(1)",
		"file:///etc/passwd",
		// A host does not make another scheme acceptable.
		"ftp://example.com/file",
		"javascript://example.com/%0Aalert(1)",
		"https://user:pw@example.com/",
		"https://user@example.com/",
		"https:///path",
		"https://exa mple.com",
		"https://example.com/a b",
		"https://example.com/\x7f",
	} {
		got, err := acp.ParseElicitationURL(raw)
		ve, ok := errors.AsType[*acp.ValidationError](err)
		if !ok || ve.Type != "CreateElicitationUrl" || ve.Field != "url" || ve.Rule != acp.RuleFormat || ve.Value != nil {
			t.Errorf("ParseElicitationURL(%q) = %+v, %v; want a RuleFormat *ValidationError of url, with no Value", raw, got, err)
			continue
		}
		if raw != "" && strings.Contains(ve.Error(), raw) {
			t.Errorf("ParseElicitationURL(%q): the message %q holds the URL", raw, ve.Error())
		}
		if strings.Contains(ve.Error(), "pw") {
			t.Errorf("ParseElicitationURL(%q): the message %q holds the credential", raw, ve.Error())
		}
	}
	for _, tc := range []struct {
		raw              string
		host             string
		secure, punycode bool
	}{
		{"https://example.com/a?b=1", "example.com", true, false},
		{"http://localhost:8080/cb", "localhost", false, false},
		{"https://XN--80AK6AA92E.com/", "xn--80ak6aa92e.com", true, true},
		{"https://sub.xn--bcher-kva.example/", "sub.xn--bcher-kva.example", true, true},
		{"https://[::1]:8443/", "::1", true, false},
	} {
		got, err := acp.ParseElicitationURL(tc.raw)
		if err != nil {
			t.Errorf("ParseElicitationURL(%q) = %v", tc.raw, err)
			continue
		}
		if got.Host != tc.host || got.Secure != tc.secure || got.Punycode != tc.punycode || got.URL == nil || got.URL.String() == "" {
			t.Errorf("ParseElicitationURL(%q) = %+v, want host %q, secure %v, punycode %v", tc.raw, got, tc.host, tc.secure, tc.punycode)
		}
	}
}
