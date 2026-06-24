// Package auth contains the upgrade-time gates: origin checking,
// authentication, connection limits, and subprotocol negotiation.
package auth

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gnemade360/go-server/websocket/internal/types"
)

// SameOriginChecker accepts the upgrade only when Origin is missing or
// matches the request Host. It is the default and defends against CSWSH.
type SameOriginChecker struct{}

func (SameOriginChecker) CheckOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	host := originHost(origin)
	if host == "" {
		return false
	}
	return strings.EqualFold(host, r.Host)
}

// AllowOrigins returns an OriginChecker that allows the listed origins
// exactly. Comparison is case-insensitive on the host.
func AllowOrigins(origins ...string) types.OriginChecker {
	allowed := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		if h := originHost(o); h != "" {
			allowed[strings.ToLower(h)] = struct{}{}
		}
	}
	return allowList{hosts: allowed}
}

type allowList struct {
	hosts map[string]struct{}
}

func (a allowList) CheckOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	_, ok := a.hosts[strings.ToLower(originHost(origin))]
	return ok
}

// AllowAllOrigins bypasses CSWSH protection. Do not use with cookie auth
// in production.
func AllowAllOrigins() types.OriginChecker { return allowAll{} }

type allowAll struct{}

func (allowAll) CheckOrigin(*http.Request) bool { return true }

// OriginCheckerFunc adapts a function to OriginChecker.
type OriginCheckerFunc func(*http.Request) bool

func (f OriginCheckerFunc) CheckOrigin(r *http.Request) bool { return f(r) }

func originHost(origin string) string {
	if u, err := url.Parse(origin); err == nil && u.Host != "" {
		return u.Host
	}
	if i := strings.Index(origin, "://"); i >= 0 {
		return origin[i+3:]
	}
	return origin
}
