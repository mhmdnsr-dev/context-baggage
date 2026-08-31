package githubsync

import "errors"

var (
	// ErrInvalidLocator reports a repository locator outside the intentionally
	// narrow GitHub.com grammar accepted by managed synchronization.
	ErrInvalidLocator = errors.New("unsupported GitHub repository URL")

	// ErrCredentialLocator reports userinfo in an HTTPS repository URL. The
	// error deliberately omits the rejected input so credentials cannot leak.
	ErrCredentialLocator = errors.New("repository URL contains credentials; use SSH or your configured Git credential helper instead")

	// ErrGitUnavailable reports that the system Git executable was not found.
	ErrGitUnavailable = errors.New("git is not installed")

	// ErrDestinationMismatch reports that Git configuration changed an
	// effective fetch or push target away from the configured repository.
	ErrDestinationMismatch = errors.New("repository target changed unexpectedly")

	// ErrTransportUnavailable reports a sanitized Git transport failure. Raw
	// subprocess output is never attached because it may contain secrets.
	ErrTransportUnavailable = errors.New("repository could not be reached")

	// ErrNetworkUnavailable reports a sanitized HTTP transport failure.
	ErrNetworkUnavailable = errors.New("network unavailable")

	// ErrPrivacyUnverifiable reports an ambiguous or malformed public lookup.
	ErrPrivacyUnverifiable = errors.New("repository visibility could not be verified")
)
