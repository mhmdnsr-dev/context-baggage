package agents

import (
	"regexp"
	"strings"
)

var secretKey = regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password|passwd|authorization|cookie|credential|pat)`)
var secretValue = regexp.MustCompile(`(?i)(sk-[a-z0-9_-]{10,}|gh[pousr]_[a-z0-9_]{10,}|xox[baprs]-[a-z0-9-]{10,}|bearer\s+[a-z0-9._-]{10,})`)

func RedactKeyValue(key, value string) string {
	// Agent config files may contain credentials. When uncertain, redact rather
	// than risk persisting a token into Context Baggage inventory.
	if secretKey.MatchString(key) || secretValue.MatchString(value) {
		return "[REDACTED]"
	}
	if len(value) > 160 {
		return value[:160] + "..."
	}
	return value
}

func CountLikelyServers(content string) string {
	lower := strings.ToLower(content)
	switch {
	case strings.Contains(lower, "mcpservers") || strings.Contains(lower, "mcp_servers"):
		return "configured"
	default:
		return "unknown"
	}
}
