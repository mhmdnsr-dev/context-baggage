package agents

import "testing"

func TestRedactKeyValue(t *testing.T) {
	for _, tc := range []struct {
		key   string
		value string
	}{
		{"api_key", "abc"},
		{"header", "Bearer example-token-value-12345"},
		{"password", "plain"},
	} {
		if got := RedactKeyValue(tc.key, tc.value); got != "[REDACTED]" {
			t.Fatalf("expected redaction for %#v, got %q", tc, got)
		}
	}
}
