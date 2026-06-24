// Package ops contains the observability glue: metrics recording for
// the WebSocket subsystem and the health check function.
package ops

import (
	"sync"

	"github.com/gnemade360/go-server/metrics"
	"github.com/gnemade360/go-server/websocket/internal/types"
)

// Metrics is the recorder wired into the conn package. Construct with
// NewMetrics and pass into the upgrade flow.
type Metrics struct {
	mu sync.Mutex

	active          *metrics.Gauge
	messagesIn      *metrics.Counter
	messagesOut     *metrics.Counter
	bytesIn         *metrics.Histogram
	bytesOut        *metrics.Histogram
	handshakeMillis *metrics.Histogram
	handshakeErrors *metrics.Counter
	closesByCode    map[int]*metrics.Counter
	registry        *metrics.Registry
}

// NewMetrics registers WebSocket metrics with r and returns a recorder.
// Pass nil for r to get a no-op recorder.
func NewMetrics(r *metrics.Registry) *Metrics {
	if r == nil {
		return nil
	}
	byteBuckets := []float64{64, 256, 1024, 4096, 16384, 65536, 262144, 1048576}
	durBuckets := []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500}
	return &Metrics{
		registry:        r,
		active:          r.NewGauge("ws_active_connections", "currently open WebSocket connections", nil),
		messagesIn:      r.NewCounter("ws_messages_in_total", "messages received from clients", nil),
		messagesOut:     r.NewCounter("ws_messages_out_total", "messages sent to clients", nil),
		bytesIn:         r.NewHistogram("ws_bytes_in", "inbound message size in bytes", nil, byteBuckets),
		bytesOut:        r.NewHistogram("ws_bytes_out", "outbound message size in bytes", nil, byteBuckets),
		handshakeMillis: r.NewHistogram("ws_handshake_duration_ms", "WebSocket upgrade duration in milliseconds", nil, durBuckets),
		handshakeErrors: r.NewCounter("ws_handshake_errors_total", "rejected upgrade attempts", nil),
		closesByCode:    make(map[int]*metrics.Counter),
	}
}

// RecordHandshakeError implements the handshake-side counter.
func (m *Metrics) RecordHandshakeError() {
	if m == nil {
		return
	}
	m.handshakeErrors.Inc()
}

// RecordHandshakeDuration logs the upgrade duration in milliseconds.
func (m *Metrics) RecordHandshakeDuration(ms float64) {
	if m == nil {
		return
	}
	m.handshakeMillis.Observe(ms)
}

// RecordConnOpened increments the active gauge.
func (m *Metrics) RecordConnOpened() {
	if m == nil {
		return
	}
	m.active.Inc()
}

// RecordConnClosed decrements the active gauge and increments the
// per-code close counter.
func (m *Metrics) RecordConnClosed(code int) {
	if m == nil {
		return
	}
	m.active.Dec()
	m.mu.Lock()
	c, ok := m.closesByCode[code]
	if !ok {
		c = m.registry.NewCounter("ws_close_total", "connection terminations by close code", map[string]string{"code": codeLabel(code)})
		m.closesByCode[code] = c
	}
	m.mu.Unlock()
	c.Inc()
}

// RecordMessageIn implements types.MetricsRecorder.
func (m *Metrics) RecordMessageIn(size int) {
	if m == nil {
		return
	}
	m.messagesIn.Inc()
	m.bytesIn.Observe(float64(size))
}

// RecordMessageOut implements types.MetricsRecorder.
func (m *Metrics) RecordMessageOut(size int) {
	if m == nil {
		return
	}
	m.messagesOut.Inc()
	m.bytesOut.Observe(float64(size))
}

func codeLabel(code int) string {
	switch code {
	case types.CloseNormalClosure:
		return "1000"
	case types.CloseGoingAway:
		return "1001"
	case types.CloseProtocolError:
		return "1002"
	case types.CloseUnsupportedData:
		return "1003"
	case types.CloseInvalidFramePayload:
		return "1007"
	case types.ClosePolicyViolation:
		return "1008"
	case types.CloseMessageTooBig:
		return "1009"
	case types.CloseInternalServerErr:
		return "1011"
	case types.CloseRateLimitExceeded:
		return "4008"
	case types.CloseQuotaExceeded:
		return "4029"
	default:
		return "other"
	}
}
