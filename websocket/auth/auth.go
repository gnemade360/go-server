package auth

import (
	"net/http"
	"strings"

	"github.com/gnemade360/go-server/websocket/internal/types"
)

// DenyAll rejects every upgrade. It is the default Authenticator so
// callers must opt into open access via AllowAnonymous.
type DenyAll struct{}

func (DenyAll) Authenticate(*http.Request) (types.Principal, error) {
	return types.Principal{}, types.ErrUnauthorized
}

// AllowAnonymous accepts every upgrade with an empty Principal.
type AllowAnonymous struct{}

func (AllowAnonymous) Authenticate(*http.Request) (types.Principal, error) {
	return types.Principal{ID: "anonymous"}, nil
}

// AuthenticatorFunc adapts a function to Authenticator.
type AuthenticatorFunc func(*http.Request) (types.Principal, error)

func (f AuthenticatorFunc) Authenticate(r *http.Request) (types.Principal, error) { return f(r) }

// BearerTokenVerifier validates a bearer token.
type BearerTokenVerifier func(token string) (types.Principal, error)

// BearerToken returns an Authenticator that extracts a bearer token
// from (in order) the Authorization header, a "bearer.<token>"
// Sec-WebSocket-Protocol entry, or the access_token query parameter.
func BearerToken(verify BearerTokenVerifier) types.Authenticator {
	if verify == nil {
		panic("websocket/auth: BearerToken requires a verifier")
	}
	return bearerAuth{verify: verify}
}

type bearerAuth struct {
	verify BearerTokenVerifier
}

func (b bearerAuth) Authenticate(r *http.Request) (types.Principal, error) {
	if token := extractAuthorizationBearer(r); token != "" {
		return b.verify(token)
	}
	if token := extractSubprotocolBearer(r); token != "" {
		return b.verify(token)
	}
	if token := r.URL.Query().Get("access_token"); token != "" {
		return b.verify(token)
	}
	return types.Principal{}, types.ErrUnauthorized
}

// CookieSessionLookup resolves a session cookie value to a Principal.
type CookieSessionLookup func(sessionID string) (types.Principal, error)

// CookieSession reads a session cookie and resolves it through lookup.
// Pair with SameOriginChecker (the default) to defeat CSWSH.
func CookieSession(cookieName string, lookup CookieSessionLookup) types.Authenticator {
	if cookieName == "" {
		panic("websocket/auth: CookieSession requires a cookie name")
	}
	if lookup == nil {
		panic("websocket/auth: CookieSession requires a lookup")
	}
	return cookieAuth{name: cookieName, lookup: lookup}
}

type cookieAuth struct {
	name   string
	lookup CookieSessionLookup
}

func (c cookieAuth) Authenticate(r *http.Request) (types.Principal, error) {
	cookie, err := r.Cookie(c.name)
	if err != nil || cookie.Value == "" {
		return types.Principal{}, types.ErrUnauthorized
	}
	return c.lookup(cookie.Value)
}

func extractAuthorizationBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

func extractSubprotocolBearer(r *http.Request) string {
	for _, raw := range r.Header.Values("Sec-WebSocket-Protocol") {
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			const prefix = "bearer."
			if len(part) > len(prefix) && strings.EqualFold(part[:len(prefix)], prefix) {
				return part[len(prefix):]
			}
		}
	}
	return ""
}
