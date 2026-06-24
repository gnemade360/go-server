package auth

import (
	"net/http"
	"strings"
)

// NegotiateSubprotocol picks the first non-bearer client-offered
// subprotocol that the server supports. The order respects client
// preference.
func NegotiateSubprotocol(r *http.Request, supported []string) string {
	if len(supported) == 0 {
		return ""
	}
	for _, raw := range r.Header.Values("Sec-WebSocket-Protocol") {
		for _, part := range strings.Split(raw, ",") {
			offered := strings.TrimSpace(part)
			if offered == "" || strings.HasPrefix(strings.ToLower(offered), "bearer.") {
				continue
			}
			for _, s := range supported {
				if strings.EqualFold(offered, s) {
					return s
				}
			}
		}
	}
	return ""
}
