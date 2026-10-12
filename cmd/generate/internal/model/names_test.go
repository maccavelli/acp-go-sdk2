package model

import (
	"strings"
	"testing"
)

// TestParamName pins the parameter names of
// 0014-MADR-union-constructors-take-every-required-key.md: an identifier
// key keeps its text; a key that is not one is put in lower camel case, its
// first word lowered at its leading capitals (L1, the table of finding 12);
// a Go keyword or predeclared identifier gets Value (N1); and a key that
// gives no identifier is refused.
func TestParamName(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ key, want string }{
		{"planId", "planId"},
		{"id", "id"},
		{"currentValue", "currentValue"},
		{"_meta", "_meta"},
		{"ID", "ID"},
		{"foo-bar", "fooBar"},
		{"content-type", "contentType"},
		{"URL-path", "urlPath"},
		{"x-API-key", "xAPIKey"},
		{"HTTPServer-url", "httpServerUrl"},
		{"XMLHttp-request", "xmlHttpRequest"},
		{"HTTP2-server", "http2Server"},
		{"api.version", "apiVersion"},
		{"ACME-id", "acmeId"},
		{"$schema", "schema"},
		{"type", "typeValue"},
		{"const", "constValue"},
		{"range", "rangeValue"},
		{"$type", "typeValue"},
		{"error", "errorValue"},
		{"string", "stringValue"},
		{"any", "anyValue"},
		{"min", "minValue"},
		{"nil", "nilValue"},
	} {
		got, err := paramName("U variant 1", tt.key)
		if err != nil || got != tt.want {
			t.Errorf("paramName(%q) = %q, %v; want %q", tt.key, got, err, tt.want)
		}
	}
	for _, key := range []string{"$", "--", "2fa", "9-lives"} {
		got, err := paramName("U variant 1", key)
		if err == nil || !strings.Contains(err.Error(), "U variant 1") || !strings.Contains(err.Error(), "cannot be a Go parameter name") {
			t.Errorf("paramName(%q) = %q, %v; want a refusal naming the owner", key, got, err)
		}
	}
}
