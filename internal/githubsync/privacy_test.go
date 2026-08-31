package githubsync

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClassifyPrivacyVerifiesExactPublicRepository(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assertPrivacyRequest(t, request)
		_, _ = fmt.Fprint(w, `{"full_name":"Owner/Repo","private":false,"visibility":"public"}`)
	}))
	defer server.Close()

	classification, err := classifyPrivacy(context.Background(), mustLocator(t, "https://github.com/owner/repo"), successfulPrivacyConfig(server))
	if err != nil || classification != VerifiedPublic {
		t.Fatalf("expected verified public, got %s, %v", classification, err)
	}
}

func TestClassifyPrivacyRequiresExactPublicIdentity(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"unexpected identity", `{"full_name":"other/repo","private":false,"visibility":"public"}`},
		{"private response", `{"full_name":"owner/repo","private":true,"visibility":"private"}`},
		{"missing fields", `{"full_name":"owner/repo"}`},
		{"malformed", `{`},
		{"trailing JSON", `{"full_name":"owner/repo","private":false,"visibility":"public"}{}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, test.body)
			}))
			defer server.Close()
			classification, err := classifyPrivacy(context.Background(), mustLocator(t, "https://github.com/owner/repo"), successfulPrivacyConfig(server))
			if classification != Unverifiable || !errors.Is(err, ErrPrivacyUnverifiable) {
				t.Fatalf("expected unverifiable, got %s, %v", classification, err)
			}
		})
	}
}

func TestClassifyPrivacyVerifiesNonPublicOnlyWithGitReadability(t *testing.T) {
	server := statusServer(http.StatusNotFound)
	defer server.Close()
	config := successfulPrivacyConfig(server)
	classification, err := classifyPrivacy(context.Background(), mustLocator(t, "git@github.com:owner/repo.git"), config)
	if err != nil || classification != VerifiedNonPublic {
		t.Fatalf("expected verified non-public, got %s, %v", classification, err)
	}

	config.git.readable = func(context.Context, Locator) error { return ErrTransportUnavailable }
	classification, err = classifyPrivacy(context.Background(), mustLocator(t, "git@github.com:owner/repo.git"), config)
	if classification != Unverifiable || !errors.Is(err, ErrTransportUnavailable) {
		t.Fatalf("expected unreadable repository to remain unverifiable, got %s, %v", classification, err)
	}
}

func TestClassifyPrivacyRefusesTargetMismatchBeforeHTTP(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	config := successfulPrivacyConfig(server)
	config.git.verifyTarget = func(context.Context, Locator) error { return ErrDestinationMismatch }
	classification, err := classifyPrivacy(context.Background(), mustLocator(t, "https://github.com/owner/repo"), config)
	if classification != Unverifiable || !errors.Is(err, ErrDestinationMismatch) {
		t.Fatalf("expected destination mismatch, got %s, %v", classification, err)
	}
	if requests.Load() != 0 {
		t.Fatal("privacy HTTP request ran after target mismatch")
	}
}

func TestClassifyPrivacyFailsClosedForAmbiguousStatuses(t *testing.T) {
	statuses := []int{301, 302, 307, 308, 403, 429, 500, 502, 418}
	for _, status := range statuses {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := statusServer(status)
			defer server.Close()
			classification, err := classifyPrivacy(context.Background(), mustLocator(t, "https://github.com/owner/repo"), successfulPrivacyConfig(server))
			if classification != Unverifiable || !errors.Is(err, ErrPrivacyUnverifiable) {
				t.Fatalf("expected status %d to be unverifiable, got %s, %v", status, classification, err)
			}
		})
	}
}

func TestPrivacyClientDoesNotFollowRedirect(t *testing.T) {
	var redirectedRequests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirectedRequests.Add(1)
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		http.Redirect(w, request, target.URL, http.StatusMovedPermanently)
	}))
	defer server.Close()
	classification, err := classifyPrivacy(context.Background(), mustLocator(t, "https://github.com/owner/repo"), successfulPrivacyConfig(server))
	if classification != Unverifiable || !errors.Is(err, ErrPrivacyUnverifiable) {
		t.Fatalf("expected redirect to be unverifiable, got %s, %v", classification, err)
	}
	if redirectedRequests.Load() != 0 {
		t.Fatal("privacy client followed redirect")
	}
}

func TestClassifyPrivacyRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, strings.Repeat("x", maxPrivacyBodyBytes+1))
	}))
	defer server.Close()
	classification, err := classifyPrivacy(context.Background(), mustLocator(t, "https://github.com/owner/repo"), successfulPrivacyConfig(server))
	if classification != Unverifiable || !errors.Is(err, ErrPrivacyUnverifiable) {
		t.Fatalf("expected oversized response to be unverifiable, got %s, %v", classification, err)
	}
}

func TestClassifyPrivacyHandlesTimeoutAndNetworkFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	config := successfulPrivacyConfig(server)
	config.client.Timeout = 50 * time.Millisecond
	started := time.Now()
	classification, err := classifyPrivacy(context.Background(), mustLocator(t, "https://github.com/owner/repo"), config)
	server.Close()
	if classification != Unverifiable || !errors.Is(err, ErrNetworkUnavailable) {
		t.Fatalf("expected timeout to be unverifiable, got %s, %v", classification, err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("HTTP timeout returned too slowly: %s", elapsed)
	}

	closedServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	closedURL := closedServer.URL
	closedServer.Close()
	config = successfulPrivacyConfig(nil)
	config.apiBaseURL = closedURL
	classification, err = classifyPrivacy(context.Background(), mustLocator(t, "https://github.com/owner/repo"), config)
	if classification != Unverifiable || !errors.Is(err, ErrNetworkUnavailable) {
		t.Fatalf("expected network failure to be unverifiable, got %s, %v", classification, err)
	}
}

func TestClassifyPrivacyRejectsMalformedEndpoint(t *testing.T) {
	config := successfulPrivacyConfig(nil)
	config.apiBaseURL = ":"
	classification, err := classifyPrivacy(context.Background(), mustLocator(t, "https://github.com/owner/repo"), config)
	if classification != Unverifiable || !errors.Is(err, ErrPrivacyUnverifiable) {
		t.Fatalf("expected malformed endpoint to be unverifiable, got %s, %v", classification, err)
	}
}

func TestClassifyPrivacyRejectsUnparsedLocatorBeforeIO(t *testing.T) {
	t.Setenv(gitHelperEnvironment, "secret-error")
	classification, err := ClassifyPrivacy(context.Background(), helperGitRunner(), Locator{})
	if classification != Unverifiable || !errors.Is(err, ErrInvalidLocator) {
		t.Fatalf("expected invalid locator before I/O, got %s, %v", classification, err)
	}
}

func TestClassifyPrivacyHonorsCancelledContext(t *testing.T) {
	server := statusServer(http.StatusNotFound)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	classification, err := classifyPrivacy(ctx, mustLocator(t, "https://github.com/owner/repo"), successfulPrivacyConfig(server))
	if classification != Unverifiable || !errors.Is(err, ErrNetworkUnavailable) {
		t.Fatalf("expected cancelled request to be unverifiable, got %s, %v", classification, err)
	}
}

func successfulPrivacyConfig(server *httptest.Server) privacyConfig {
	baseURL := "http://127.0.0.1"
	if server != nil {
		baseURL = server.URL
	}
	return privacyConfig{
		client:     newPrivacyHTTPClient(),
		apiBaseURL: baseURL,
		git: privacyGitChecks{
			verifyTarget: func(context.Context, Locator) error { return nil },
			readable:     func(context.Context, Locator) error { return nil },
		},
	}
}

func statusServer(status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
}

func assertPrivacyRequest(t *testing.T, request *http.Request) {
	t.Helper()
	if request.URL.Path != "/repos/owner/repo" {
		t.Fatalf("unexpected privacy endpoint: %s", request.URL.Path)
	}
	if request.Header.Get("Authorization") != "" || request.Header.Get("Cookie") != "" {
		t.Fatal("privacy request included credentials")
	}
	if request.Header.Get("Accept") != githubJSONMediaType || request.Header.Get("User-Agent") != privacyUserAgent {
		t.Fatalf("missing stable privacy headers: %#v", request.Header)
	}
	if request.Header.Get("X-GitHub-Api-Version") != githubAPIVersion {
		t.Fatalf("unexpected API version: %q", request.Header.Get("X-GitHub-Api-Version"))
	}
}
