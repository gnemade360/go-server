package websocket

import (
	"net/http"
	"time"

	gws "github.com/gorilla/websocket"

	"github.com/gnemade360/go-server/websocket/auth"
	"github.com/gnemade360/go-server/websocket/conn"
	"github.com/gnemade360/go-server/websocket/internal/netutil"
	"github.com/gnemade360/go-server/websocket/internal/types"
	"github.com/gnemade360/go-server/websocket/ops"
)

// metricsRecorderOrNil returns a true nil interface when m is a nil
// *ops.Metrics, avoiding the classic typed-nil interface footgun.
func metricsRecorderOrNil(m *ops.Metrics) types.MetricsRecorder {
	if m == nil {
		return nil
	}
	return m
}

// ServeHTTP runs the fixed upgrade flow: origin → auth → limits →
// subprotocol → upgrade → register. New behaviour is added by
// satisfying one of the hook interfaces in its own subpackage and
// wiring it through Config.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	if h.isClosed() {
		h.metrics.RecordHandshakeError()
		http.Error(w, "shutting down", http.StatusServiceUnavailable)
		return
	}

	if !h.cfg.Origin.CheckOrigin(r) {
		h.metrics.RecordHandshakeError()
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}

	principal, err := h.cfg.Auth.Authenticate(r)
	if err != nil {
		h.metrics.RecordHandshakeError()
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	release, err := h.cfg.Limits.Acquire(r, principal)
	if err != nil {
		h.metrics.RecordHandshakeError()
		http.Error(w, "connection limit reached", http.StatusServiceUnavailable)
		return
	}

	subproto := auth.NegotiateSubprotocol(r, h.cfg.Subprotocols)

	upgrader := gws.Upgrader{
		HandshakeTimeout:  h.cfg.HandshakeTimeout,
		EnableCompression: h.cfg.EnableCompression,
		CheckOrigin:       func(*http.Request) bool { return true },
	}
	if subproto != "" {
		upgrader.Subprotocols = []string{subproto}
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.metrics.RecordHandshakeError()
		release()
		return
	}

	h.metrics.RecordHandshakeDuration(float64(time.Since(start).Milliseconds()))
	h.metrics.RecordConnOpened()

	c := conn.New(conn.Options{
		WS:        ws,
		Cfg:       h.cfg,
		Principal: principal,
		RemoteIP:  netutil.ClientIP(r),
		Subproto:  ws.Subprotocol(),
		Metrics:   metricsRecorderOrNil(h.metrics),
	})

	h.track(c)
	if hub := h.hubIfStarted(); hub != nil {
		hub.Register(c)
	}

	go func() {
		defer release()
		defer h.untrack(c)
		defer func() {
			if hub := h.hubIfStarted(); hub != nil {
				hub.Unregister(c)
			}
		}()
		c.Run(h.ctx)
		h.metrics.RecordConnClosed(c.LastCloseCode())
	}()
}
