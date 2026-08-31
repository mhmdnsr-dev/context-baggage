package githubsync

import (
	"errors"
	"strings"
	"testing"
)

func TestParseLocatorAcceptsLockedForms(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		transport TransportForm
		url       string
	}{
		{"ssh", "git@github.com:Owner/Repo.git", TransportSSH, "git@github.com:owner/repo.git"},
		{"https", "https://github.com/Owner/Repo", TransportHTTPS, "https://github.com/owner/repo.git"},
		{"https git suffix", "https://github.com/Owner/Repo.git", TransportHTTPS, "https://github.com/owner/repo.git"},
		{"https trailing slash", "https://github.com/Owner/Repo/", TransportHTTPS, "https://github.com/owner/repo.git"},
		{"https git suffix trailing slash", "https://github.com/Owner/Repo.git/", TransportHTTPS, "https://github.com/owner/repo.git"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			locator, err := ParseLocator(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if locator.Transport() != test.transport || locator.TransportURL() != test.url {
				t.Fatalf("unexpected transport locator: %#v", locator)
			}
			if locator.Owner() != "owner" || locator.Repository() != "repo" || locator.Identity() != "github.com/owner/repo" {
				t.Fatalf("unexpected repository identity: %#v", locator)
			}
		})
	}
}

func TestParseLocatorRejectsUnsupportedForms(t *testing.T) {
	tests := []string{
		"github.com/owner/repo",
		"http://github.com/owner/repo",
		"ssh://git@github.com/owner/repo.git",
		"git://github.com/owner/repo.git",
		"file:///owner/repo",
		"/tmp/repo",
		"https://git.example.com/owner/repo",
		"https://gitlab.com/owner/repo",
		"git@github-alias:owner/repo.git",
		"user@github.com:owner/repo.git",
		"https://github.com:443/owner/repo",
		"https://github.com:/owner/repo",
		"https://github.com/owner/repo?ref=main",
		"https://github.com/owner/repo#fragment",
		"https://github.com/owner/repo/extra",
		"https://github.com//repo",
		"https://github.com/owner/",
		"https://github.com/./repo",
		"https://github.com/owner/..",
		"https://github.com/owner%2Frewrite/repo",
		"https://github.com/owner/repo%2Fother",
		`https://github.com/owner\repo`,
		"git@github.com:owner/repo",
		"git@github.com:owner/repo.git/",
		"git@github.com:owner/repo/extra.git",
		"https://github.com/owner/repo//",
		"https://github.com/owner/repo\n",
	}
	for _, input := range tests {
		t.Run(strings.ReplaceAll(input, "/", "_"), func(t *testing.T) {
			if _, err := ParseLocator(input); !errors.Is(err, ErrInvalidLocator) {
				t.Fatalf("expected invalid locator for %q, got %v", input, err)
			}
		})
	}
}

func TestParseLocatorRejectsCredentialsWithoutEcho(t *testing.T) {
	const username = "user-example"
	const password = "password-example"
	tests := []string{
		"https://" + username + ":" + password + "@github.com/owner/repo.git",
		"https://" + username + "@github.com/owner/repo.git",
	}
	for _, input := range tests {
		t.Run(strings.ReplaceAll(strings.ReplaceAll(input, "/", "_"), ":", "_"), func(t *testing.T) {
			_, err := ParseLocator(input)
			if !errors.Is(err, ErrCredentialLocator) {
				t.Fatalf("expected credential error, got %v", err)
			}
			message := err.Error()
			if strings.Contains(message, username) || strings.Contains(message, password) || strings.Contains(message, input) {
				t.Fatalf("credential-bearing input leaked in error: %q", message)
			}
		})
	}
}

func TestParseLocatorMakesSSHAndHTTPSEquivalent(t *testing.T) {
	ssh, err := ParseLocator("git@github.com:Owner/Repo.git")
	if err != nil {
		t.Fatal(err)
	}
	https, err := ParseLocator("https://github.com/owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if ssh.Identity() != https.Identity() {
		t.Fatalf("repository identities differ: %q != %q", ssh.Identity(), https.Identity())
	}
}
