package auth

import "time"

// Method is the auth flavor for one Credentials value: empty, classic
// Application Key / Application Secret / Consumer Key, or OAuth2.
// PRD-03 §Authentication.
type Method string

// Auth method constants. Stored as the `auth_method` config key value
// (PRD-04 §Canonical key registry: <region>.auth_method).
const (
	MethodNone        Method = ""
	MethodConsumerKey Method = "consumer_key"
	MethodOAuth2      Method = "oauth2"
)

// Credentials is the value type that crosses the auth/client layer boundary.
// PRD-03 §Public symbols (store.go); PRD-06 §Canonical package registry.
//
// internal/client may import internal/auth solely to reference this type;
// the forbidigo rule in .golangci.yml forbids client from calling auth's
// lifecycle or oauth functions.
type Credentials struct {
	Region  string // region id (e.g. "ovh-eu"); see Regions()
	Profile string // profile name within the region (default: "default")
	Method  Method

	// Classic flow fields. Empty when Method != MethodConsumerKey.
	ApplicationKey    string
	ApplicationSecret string
	ConsumerKey       string

	// OAuth2 fields. Empty when Method != MethodOAuth2.
	ClientID     string
	ClientSecret string
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
}

// IsZero reports whether c equals the zero value, used to enforce the
// "nil-error + zero Credentials is FORBIDDEN" contract on LoadCredentials.
//
// Note: this is a struct-equality comparison. time.Time fields with a
// non-nil Location (e.g., the result of time.Time{}.UTC()) will NOT compare
// equal to the bare zero time.Time. Callers that mean "no Expiry" must
// leave Expiry at its zero value (do not apply .UTC()). Today this is
// consistent: only successful OAuth2 token responses populate Expiry,
// always to a non-zero time.
func (c Credentials) IsZero() bool {
	return c == Credentials{}
}
