package githubsync

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

// TransportForm identifies one accepted GitHub transport syntax.
type TransportForm string

const (
	// TransportSSH is the conventional git@github.com SCP-style transport.
	TransportSSH TransportForm = "ssh"
	// TransportHTTPS is the credential-helper-compatible HTTPS transport.
	TransportHTTPS TransportForm = "https"
)

var repositorySegment = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Locator is a parsed, credential-free GitHub.com repository locator.
// Owner and Repository are canonical lowercase values used by both Git target
// binding and the GitHub REST privacy lookup.
type Locator struct {
	transport  TransportForm
	url        string
	owner      string
	repository string
	identity   string
}

// Transport returns the accepted transport form.
func (l Locator) Transport() TransportForm { return l.transport }

// TransportURL returns the canonical credential-free Git remote URL.
func (l Locator) TransportURL() string { return l.url }

// Owner returns the canonical lowercase GitHub owner.
func (l Locator) Owner() string { return l.owner }

// Repository returns the canonical lowercase GitHub repository name.
func (l Locator) Repository() string { return l.repository }

// Identity returns the provisional github.com/owner/repository identity.
func (l Locator) Identity() string { return l.identity }

// ParseLocator accepts only the locked GitHub.com SSH and HTTPS forms. Strict
// parsing prevents ambiguous/path-like values from reaching Git, HTTP, errors,
// logs, or future persistence.
func ParseLocator(raw string) (Locator, error) {
	if hasControl(raw) || strings.Contains(raw, `\`) {
		return Locator{}, ErrInvalidLocator
	}
	if parsed, err := url.Parse(raw); err == nil && parsed.Scheme == "https" && parsed.User != nil {
		return Locator{}, ErrCredentialLocator
	}
	switch {
	case strings.HasPrefix(raw, "https://"):
		return parseHTTPSLocator(raw)
	case strings.HasPrefix(raw, "git@"):
		return parseSSHLocator(raw)
	default:
		return Locator{}, ErrInvalidLocator
	}
}

// parseHTTPSLocator validates one exact owner/repository GitHub.com URL.
func parseHTTPSLocator(raw string) (Locator, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Opaque != "" || parsed.User != nil {
		return Locator{}, ErrInvalidLocator
	}
	if !strings.EqualFold(parsed.Host, "github.com") {
		return Locator{}, ErrInvalidLocator
	}
	if parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return Locator{}, ErrInvalidLocator
	}
	escapedPath := parsed.EscapedPath()
	if strings.Contains(escapedPath, "%") || !strings.HasPrefix(escapedPath, "/") {
		return Locator{}, ErrInvalidLocator
	}
	path := strings.TrimPrefix(escapedPath, "/")
	path = strings.TrimSuffix(path, "/")
	return locatorFromPath(TransportHTTPS, path)
}

// parseSSHLocator validates only git@github.com:owner/repository.git.
func parseSSHLocator(raw string) (Locator, error) {
	const prefix = "git@github.com:"
	if !strings.HasPrefix(raw, prefix) || !strings.HasSuffix(raw, ".git") {
		return Locator{}, ErrInvalidLocator
	}
	path := strings.TrimSuffix(strings.TrimPrefix(raw, prefix), ".git")
	return locatorFromPath(TransportSSH, path)
}

// locatorFromPath validates the two repository path segments and constructs a
// canonical credential-free transport URL and provisional repository identity.
func locatorFromPath(transport TransportForm, path string) (Locator, error) {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return Locator{}, ErrInvalidLocator
	}
	owner := parts[0]
	repository := strings.TrimSuffix(parts[1], ".git")
	if !validRepositorySegment(owner) || !validRepositorySegment(repository) {
		return Locator{}, ErrInvalidLocator
	}
	owner = strings.ToLower(owner)
	repository = strings.ToLower(repository)
	identity := "github.com/" + owner + "/" + repository
	transportURL := "https://github.com/" + owner + "/" + repository + ".git"
	if transport == TransportSSH {
		transportURL = "git@github.com:" + owner + "/" + repository + ".git"
	}
	return Locator{
		transport:  transport,
		url:        transportURL,
		owner:      owner,
		repository: repository,
		identity:   identity,
	}, nil
}

func (l Locator) valid() bool {
	if !validRepositorySegment(l.owner) || !validRepositorySegment(l.repository) {
		return false
	}
	if l.identity != "github.com/"+l.owner+"/"+l.repository {
		return false
	}
	expectedURL := "https://github.com/" + l.owner + "/" + l.repository + ".git"
	if l.transport == TransportSSH {
		expectedURL = "git@github.com:" + l.owner + "/" + l.repository + ".git"
	}
	return (l.transport == TransportSSH || l.transport == TransportHTTPS) && l.url == expectedURL
}

func validRepositorySegment(value string) bool {
	return value != "." && value != ".." && repositorySegment.MatchString(value)
}

func hasControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
