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

	// ErrRepositoryPublic reports a candidate proven public during explicit
	// managed destination configuration.
	ErrRepositoryPublic = errors.New("managed sync requires a non-public repository")

	// ErrManagedDestinationLost reports a previously claimed destination that is now empty.
	ErrManagedDestinationLost = errors.New("managed destination identity is no longer present")

	// ErrPublicationConflict reports an exact lease loss or a changed confirmed ref.
	ErrPublicationConflict = errors.New("managed publication lost a remote race")

	// ErrPublicationAmbiguous reports a push whose resulting remote state cannot be proven.
	ErrPublicationAmbiguous = errors.New("managed publication outcome is ambiguous")

	// ErrPullConflict reports a managed Pull that would overwrite divergent
	// local portable state without a safe shared baseline.
	ErrPullConflict = errors.New("managed pull lost a local conflict")

	// ErrApplyMismatch reports canonical application that did not produce the
	// recorded target portable identity.
	ErrApplyMismatch = errors.New("managed pull did not produce the target portable identity")

	// ErrRecoveryRequired reports an operation refused because an interrupted
	// managed Pull has left canonical state possibly partial and BASE unfinalized.
	ErrRecoveryRequired = errors.New("managed pull recovery is required")

	// ErrRecoveryAmbiguous reports recovery input that is neither the recorded
	// pre-state nor the recorded target state.
	ErrRecoveryAmbiguous = errors.New("managed pull recovery state is ambiguous")

	// ErrRecoveryBindingMismatch reports a recovery record that does not belong
	// to the active managed destination.
	ErrRecoveryBindingMismatch = errors.New("managed pull recovery record does not match the active destination")
)
