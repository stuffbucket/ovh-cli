package auth

import "errors"

// Sentinel errors. Callers MUST distinguish via errors.Is — never by string
// match. PRD-03 §Public symbols of internal/auth (store.go).
var (
	// ErrNotConfigured means no credentials exist for the (region, profile).
	ErrNotConfigured = errors.New("auth: no credentials configured for region/profile")

	// ErrCredentialsExpired means OAuth2 credentials are present but expired
	// and the caller should run RefreshOAuth2 (phase 2b iteration 3).
	ErrCredentialsExpired = errors.New("auth: stored credentials expired (OAuth2 refresh required)")

	// ErrKeyringUnavailable means the OS keyring isn't reachable; on systems
	// where default.storage=keyring and the keyring is down, callers should
	// either flip default.storage=file or rerun on a system with keyring.
	ErrKeyringUnavailable = errors.New("auth: OS keyring not available; default.storage=file forces file fallback")

	// ErrFileModeUnsafe means a credential file's mode is looser than the
	// PRD-04 §Canonical file-mode registry value; the loader refuses to
	// read until the user fixes the perms.
	ErrFileModeUnsafe = errors.New("auth: credential file mode looser than registry value; refusing to read")
)
