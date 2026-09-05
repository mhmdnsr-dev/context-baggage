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

	// ErrRepositoryIncompatible reports repository refs or contents outside the
	// dedicated managed-repository format.
	ErrRepositoryIncompatible = errors.New("repository is incompatible with managed synchronization")

	// ErrManagedMarkerInvalid reports malformed or unsupported management metadata.
	ErrManagedMarkerInvalid = errors.New("managed repository marker is invalid")

	// ErrManagedDestinationMismatch reports a marker identity other than the expected destination.
	ErrManagedDestinationMismatch = errors.New("managed destination identity does not match")

	// ErrManagedDestinationAdoptionRequired reports an initialized destination
	// observed without an already-bound expected identity.
	ErrManagedDestinationAdoptionRequired = errors.New("managed destination requires explicit adoption")

	// ErrResourceLimitExceeded reports a repository outside locked inspection bounds.
	ErrResourceLimitExceeded = errors.New("managed repository exceeds resource limits")

	// ErrPrivacyRefused reports a repository that was not freshly proven non-public.
	ErrPrivacyRefused = errors.New("managed publication requires a verified non-public repository")

	// ErrManagedDestinationLost reports a previously claimed destination that is now empty.
	ErrManagedDestinationLost = errors.New("managed destination identity is no longer present")

	// ErrPublicationConflict reports an exact lease loss or a changed confirmed ref.
	ErrPublicationConflict = errors.New("managed publication lost a remote race")

	// ErrPublicationAmbiguous reports a push whose resulting remote state cannot be proven.
	ErrPublicationAmbiguous = errors.New("managed publication outcome is ambiguous")
)
