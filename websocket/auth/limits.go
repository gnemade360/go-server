package auth

import (
	"net/http"
	"sync"

	"github.com/gnemade360/go-server/websocket/internal/netutil"
	"github.com/gnemade360/go-server/websocket/internal/types"
)

// NewDefaultLimiter returns a ConnectionLimiter enforcing the given
// caps. A zero cap disables that dimension.
func NewDefaultLimiter(globalCap, perIPCap, perUserCap int) types.ConnectionLimiter {
	return &defaultLimiter{
		globalCap:  globalCap,
		perIPCap:   perIPCap,
		perUserCap: perUserCap,
		perIP:      make(map[string]int),
		perUser:    make(map[string]int),
	}
}

type defaultLimiter struct {
	mu sync.Mutex

	globalCap, perIPCap, perUserCap int

	global  int
	perIP   map[string]int
	perUser map[string]int
}

// GlobalCap implements types.CapInspector so health checks can report
// the configured ceiling.
func (l *defaultLimiter) GlobalCap() int { return l.globalCap }

func (l *defaultLimiter) Acquire(r *http.Request, p types.Principal) (func(), error) {
	ip := netutil.ClientIP(r)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.globalCap > 0 && l.global >= l.globalCap {
		return nil, types.ErrLimitExceeded
	}
	if l.perIPCap > 0 && l.perIP[ip] >= l.perIPCap {
		return nil, types.ErrLimitExceeded
	}
	if l.perUserCap > 0 && p.ID != "" && l.perUser[p.ID] >= l.perUserCap {
		return nil, types.ErrLimitExceeded
	}

	l.global++
	l.perIP[ip]++
	if p.ID != "" {
		l.perUser[p.ID]++
	}

	released := false
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		if released {
			return
		}
		released = true
		l.global--
		if l.perIP[ip]--; l.perIP[ip] <= 0 {
			delete(l.perIP, ip)
		}
		if p.ID != "" {
			if l.perUser[p.ID]--; l.perUser[p.ID] <= 0 {
				delete(l.perUser, p.ID)
			}
		}
	}, nil
}
