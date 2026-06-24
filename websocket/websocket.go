// Package websocket provides a secure RFC 6455 WebSocket handler built
// on github.com/gorilla/websocket. Configure with websocket.Config and
// register the returned *Handler on the framework router.
//
// Internals are organized into subpackages:
//
//   - websocket/auth     — origin checking, authenticators, limits, subprotocol
//   - websocket/conn     — per-connection wrapper, codec, heartbeat, rate limit
//   - websocket/hub      — fan-out hub, rooms, broadcast helpers
//   - websocket/ops      — metrics and health glue
//   - websocket/internal — types and helpers shared across subpackages
//
// Most user code only ever sees this root package.
package websocket

import (
	"context"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	gometrics "github.com/gnemade360/go-server/metrics"
	"github.com/gnemade360/go-server/health"

	"github.com/gnemade360/go-server/websocket/auth"
	"github.com/gnemade360/go-server/websocket/conn"
	"github.com/gnemade360/go-server/websocket/hub"
	"github.com/gnemade360/go-server/websocket/internal/types"
	"github.com/gnemade360/go-server/websocket/ops"
)

// Public re-exports from internal/types.
type (
	Config             = types.Config
	Message            = types.Message
	MessageType        = types.MessageType
	Principal          = types.Principal
	OriginChecker      = types.OriginChecker
	Authenticator      = types.Authenticator
	ConnectionLimiter  = types.ConnectionLimiter
	MessageCodec       = types.MessageCodec
	MessageRateLimiter = types.MessageRateLimiter
	ConnRef            = types.ConnRef
)

// Conn is the concrete per-connection wrapper.
type Conn = conn.Conn

// Hub is the in-process fan-out plane.
type Hub = hub.Hub

// Room is a named subset of hub members.
type Room = hub.Room

// Re-exported close codes.
const (
	CloseNormalClosure       = types.CloseNormalClosure
	CloseGoingAway           = types.CloseGoingAway
	CloseProtocolError       = types.CloseProtocolError
	CloseUnsupportedData     = types.CloseUnsupportedData
	CloseInvalidFramePayload = types.CloseInvalidFramePayload
	ClosePolicyViolation     = types.ClosePolicyViolation
	CloseMessageTooBig       = types.CloseMessageTooBig
	CloseInternalServerErr   = types.CloseInternalServerErr
	CloseServiceRestart      = types.CloseServiceRestart
	CloseTryAgainLater       = types.CloseTryAgainLater
	CloseRateLimitExceeded   = types.CloseRateLimitExceeded
	CloseQuotaExceeded       = types.CloseQuotaExceeded
)

// Re-exported message types.
const (
	TextMessage   = types.TextMessage
	BinaryMessage = types.BinaryMessage
)

// Re-exported sentinel errors.
var (
	ErrHandlerClosed   = types.ErrHandlerClosed
	ErrOriginRejected  = types.ErrOriginRejected
	ErrUnauthorized    = types.ErrUnauthorized
	ErrLimitExceeded   = types.ErrLimitExceeded
	ErrSubprotocol     = types.ErrSubprotocol
	ErrWriteQueueFull  = types.ErrWriteQueueFull
	ErrMessageTooLarge = types.ErrMessageTooLarge
	ErrInvalidUTF8     = types.ErrInvalidUTF8
	ErrConnClosed      = types.ErrConnClosed
)

// Re-exported origin checkers.
type SameOriginChecker = auth.SameOriginChecker

func AllowOrigins(origins ...string) OriginChecker { return auth.AllowOrigins(origins...) }
func AllowAllOrigins() OriginChecker               { return auth.AllowAllOrigins() }

// OriginCheckerFunc adapts a function to OriginChecker.
type OriginCheckerFunc = auth.OriginCheckerFunc

// Re-exported authenticators.
type (
	DenyAll          = auth.DenyAll
	AllowAnonymous   = auth.AllowAnonymous
	AuthenticatorFunc = auth.AuthenticatorFunc
)

// BearerTokenVerifier is the verify function for BearerTokenAuth.
type BearerTokenVerifier = auth.BearerTokenVerifier

// BearerTokenAuth returns an Authenticator that extracts a bearer
// token from the Authorization header, a bearer.<token> subprotocol
// entry, or the access_token query parameter (in that order).
func BearerTokenAuth(verify BearerTokenVerifier) Authenticator {
	return auth.BearerToken(verify)
}

// CookieSessionLookup resolves a session cookie value to a Principal.
type CookieSessionLookup = auth.CookieSessionLookup

// CookieSessionAuth reads a session cookie and resolves it through lookup.
func CookieSessionAuth(cookieName string, lookup CookieSessionLookup) Authenticator {
	return auth.CookieSession(cookieName, lookup)
}

// NewDefaultLimiter returns a ConnectionLimiter enforcing the given caps.
func NewDefaultLimiter(globalCap, perIPCap, perUserCap int) ConnectionLimiter {
	return auth.NewDefaultLimiter(globalCap, perIPCap, perUserCap)
}

// NewTokenBucketRateLimiter returns a per-connection rate limiter.
func NewTokenBucketRateLimiter(messagesPerSecond, bytesPerSecond float64) MessageRateLimiter {
	return conn.NewTokenBucketRateLimiter(messagesPerSecond, bytesPerSecond)
}

// JSONCodec is the default codec — JSON text frames with depth limit.
type JSONCodec = conn.JSONCodec

// ValidUTF8 reports whether p is valid UTF-8 (used by the read pump).
func ValidUTF8(p []byte) bool { return conn.ValidUTF8(p) }

// Handler implements http.Handler. One Handler can serve any number of
// concurrent connections.
type Handler struct {
	cfg types.Config

	mu    sync.Mutex
	conns map[string]*conn.Conn

	hubOnce sync.Once
	hub     atomic.Pointer[hub.Hub]

	metrics *ops.Metrics

	closed chan struct{}

	ctx       context.Context
	cancelCtx context.CancelFunc
}

// NewHandler returns a Handler configured per cfg with safe defaults
// filled in for unset fields.
func NewHandler(cfg Config) *Handler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Handler{
		cfg:       withDefaults(cfg),
		conns:     make(map[string]*conn.Conn),
		closed:    make(chan struct{}),
		ctx:       ctx,
		cancelCtx: cancel,
	}
}

// Config returns the resolved configuration.
func (h *Handler) Config() Config { return h.cfg }

// Hub returns the fan-out hub for this handler, creating it on first
// access. All subsequent upgrades register their connection here.
func (h *Handler) Hub() *Hub {
	h.hubOnce.Do(func() {
		h.hub.Store(hub.New())
	})
	return h.hub.Load()
}

func (h *Handler) hubIfStarted() *Hub {
	return h.hub.Load()
}

// RegisterMetrics wires the handler into r. Pass nil to detach.
func (h *Handler) RegisterMetrics(r *gometrics.Registry) {
	h.metrics = ops.NewMetrics(r)
}

// HealthCheck returns a health.CheckFunc that reports the WebSocket
// subsystem state.
func (h *Handler) HealthCheck() health.CheckFunc {
	return ops.Check(func() ops.HealthSnapshot {
		hub := h.hubIfStarted()
		return ops.HealthSnapshot{
			ShuttingDown: h.isClosed(),
			HubRunning:   hub != nil && hub.IsRunning(),
			HubStopped:   hub != nil && !hub.IsRunning(),
			Active:       len(h.activeConns()),
			Limiter:      h.cfg.Limits,
		}
	})
}

// Shutdown closes every open connection with code 1001 and waits for
// drain. Idempotent. Bounded by ctx; no deadline defaults to 30s.
func (h *Handler) Shutdown(ctx context.Context) error {
	select {
	case <-h.closed:
		return nil
	default:
		close(h.closed)
	}

	for _, c := range h.activeConns() {
		_ = c.Close(CloseGoingAway, "server shutting down")
	}

	deadline := time.NewTimer(remaining(ctx))
	defer deadline.Stop()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if len(h.activeConns()) == 0 {
			break
		}
		select {
		case <-ctx.Done():
			h.cancelCtx()
			return ctx.Err()
		case <-deadline.C:
			h.cancelCtx()
			return context.DeadlineExceeded
		case <-ticker.C:
		}
	}

	h.cancelCtx()
	if hub := h.hubIfStarted(); hub != nil {
		hub.Stop()
	}
	return nil
}

func (h *Handler) track(c *conn.Conn) {
	h.mu.Lock()
	h.conns[c.ID()] = c
	h.mu.Unlock()
}

func (h *Handler) untrack(c *conn.Conn) {
	h.mu.Lock()
	delete(h.conns, c.ID())
	h.mu.Unlock()
}

func (h *Handler) activeConns() []*conn.Conn {
	h.mu.Lock()
	out := make([]*conn.Conn, 0, len(h.conns))
	for _, c := range h.conns {
		out = append(out, c)
	}
	h.mu.Unlock()
	return out
}

func (h *Handler) isClosed() bool {
	select {
	case <-h.closed:
		return true
	default:
		return false
	}
}

func remaining(ctx context.Context) time.Duration {
	if dl, ok := ctx.Deadline(); ok {
		return time.Until(dl)
	}
	return 30 * time.Second
}

func withDefaults(c Config) Config {
	if c.Origin == nil {
		c.Origin = SameOriginChecker{}
	}
	if c.Auth == nil {
		c.Auth = DenyAll{}
	}
	if c.Limits == nil {
		c.Limits = NewDefaultLimiter(1000, 10, 0)
	}
	if c.Codec == nil {
		c.Codec = JSONCodec{MaxDepth: 64}
	}
	if c.ReadLimit <= 0 {
		c.ReadLimit = 1 << 20
	}
	if c.MessageReassemblyLimit <= 0 {
		c.MessageReassemblyLimit = 1 << 20
	}
	if c.WriteBuffer <= 0 {
		c.WriteBuffer = 64
	}
	if c.PingInterval <= 0 {
		c.PingInterval = 30 * time.Second
	}
	if c.PongTimeout <= 0 {
		c.PongTimeout = 60 * time.Second
	}
	if c.HandshakeTimeout <= 0 {
		c.HandshakeTimeout = 10 * time.Second
	}
	if c.Logger == nil {
		c.Logger = discardLogger
	}
	return c
}

var discardLogger = log.New(discardWriter{}, "", 0)

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

var _ http.Handler = (*Handler)(nil)
