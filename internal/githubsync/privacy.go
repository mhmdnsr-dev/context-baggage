package githubsync

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	githubAPIBaseURL    = "https://api.github.com"
	githubAPIVersion    = "2026-03-10"
	privacyHTTPTimeout  = 10 * time.Second
	maxPrivacyBodyBytes = 1024 * 1024
	privacyUserAgent    = "ctx-bag"
	githubJSONMediaType = "application/vnd.github+json"
)

// PrivacyClassification is the fail-closed public visibility result.
type PrivacyClassification string

const (
	// VerifiedPublic means an exact unauthenticated public repository response.
	VerifiedPublic PrivacyClassification = "VerifiedPublic"
	// VerifiedNonPublic means bound authenticated Git succeeded and the direct
	// unauthenticated repository lookup returned 404.
	VerifiedNonPublic PrivacyClassification = "VerifiedNonPublic"
	// Unverifiable means no safe public/non-public conclusion can be drawn.
	Unverifiable PrivacyClassification = "Unverifiable"
)

type privacyGitChecks struct {
	verifyTarget func(context.Context, Locator) error
	readable     func(context.Context, Locator) error
}

type privacyConfig struct {
	client     *http.Client
	apiBaseURL string
	git        privacyGitChecks
}

// ClassifyPrivacy binds Git fetch/push targets to the strict locator, then
// performs a direct unauthenticated lookup. A 404 becomes VerifiedNonPublic
// only after authenticated Git readability also succeeds.
func ClassifyPrivacy(ctx context.Context, git GitRunner, locator Locator) (PrivacyClassification, error) {
	if !locator.valid() {
		return Unverifiable, ErrInvalidLocator
	}
	return classifyPrivacy(ctx, locator, privacyConfig{
		client:     newPrivacyHTTPClient(),
		apiBaseURL: githubAPIBaseURL,
		git: privacyGitChecks{
			verifyTarget: git.VerifyTargetBinding,
			readable:     git.readRepository,
		},
	})
}

func classifyPrivacy(ctx context.Context, locator Locator, config privacyConfig) (PrivacyClassification, error) {
	if err := config.git.verifyTarget(ctx, locator); err != nil {
		return Unverifiable, err
	}
	observation, err := lookupPublicRepository(ctx, locator, config)
	if err != nil {
		return Unverifiable, err
	}
	switch observation {
	case publicRepository:
		return VerifiedPublic, nil
	case publiclyUnresolvable:
		if err := config.git.readable(ctx, locator); err != nil {
			return Unverifiable, err
		}
		return VerifiedNonPublic, nil
	default:
		return Unverifiable, ErrPrivacyUnverifiable
	}
}

type publicObservation int

const (
	ambiguousRepository publicObservation = iota
	publicRepository
	publiclyUnresolvable
)

// lookupPublicRepository never follows redirects and never sends credentials.
// Only an exact public 200 or a direct 404 has security meaning.
func lookupPublicRepository(ctx context.Context, locator Locator, config privacyConfig) (publicObservation, error) {
	endpoint, err := repositoryAPIEndpoint(config.apiBaseURL, locator)
	if err != nil {
		return ambiguousRepository, ErrPrivacyUnverifiable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ambiguousRepository, ErrPrivacyUnverifiable
	}
	request.Header.Set("Accept", githubJSONMediaType)
	request.Header.Set("User-Agent", privacyUserAgent)
	request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	response, err := config.client.Do(request)
	if err != nil {
		return ambiguousRepository, ErrNetworkUnavailable
	}
	defer func() { _ = response.Body.Close() }()
	switch response.StatusCode {
	case http.StatusNotFound:
		return publiclyUnresolvable, nil
	case http.StatusOK:
		return parsePublicResponse(response, locator)
	default:
		return ambiguousRepository, ErrPrivacyUnverifiable
	}
}

func repositoryAPIEndpoint(baseURL string, locator Locator) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", ErrPrivacyUnverifiable
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/repos/" + locator.owner + "/" + locator.repository
	base.RawQuery = ""
	base.Fragment = ""
	return base.String(), nil
}

type publicRepositoryResponse struct {
	FullName   string `json:"full_name"`
	Private    *bool  `json:"private"`
	Visibility string `json:"visibility"`
}

// parsePublicResponse requires the exact repository identity and explicit
// public fields. Missing, malformed, oversized, or trailing data fails closed.
func parsePublicResponse(response *http.Response, locator Locator) (publicObservation, error) {
	if response.ContentLength > maxPrivacyBodyBytes {
		return ambiguousRepository, ErrPrivacyUnverifiable
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxPrivacyBodyBytes+1))
	if err != nil || len(body) > maxPrivacyBodyBytes {
		return ambiguousRepository, ErrPrivacyUnverifiable
	}
	var repository publicRepositoryResponse
	if err := json.Unmarshal(body, &repository); err != nil {
		return ambiguousRepository, ErrPrivacyUnverifiable
	}
	public := repository.Private != nil && !*repository.Private && repository.Visibility == "public"
	expectedName := locator.owner + "/" + locator.repository
	if !public || !strings.EqualFold(repository.FullName, expectedName) {
		return ambiguousRepository, ErrPrivacyUnverifiable
	}
	return publicRepository, nil
}

func newPrivacyHTTPClient() *http.Client {
	return &http.Client{
		Timeout: privacyHTTPTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
