// Package conn wraps an upgraded gorilla connection with serialized
// writes, bounded outbound queue, read/write pumps, ping/pong, rate
// limiting, UTF-8 validation, and codec-based message exchange.
package conn

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	gws "github.com/gorilla/websocket"

	"github.com/gnemade360/go-server/websocket/internal/types"
)

// Conn is the framework wrapper around *gws.Conn. All writes serialize
// under a mutex; reads run on a single goroutine.
type Conn struct {
	id        string
	principal types.Principal
	remoteIP  string
	subproto  string

	ws *gws.Conn

	cfg      types.Config
	metrics  types.MetricsRecorder
	onClose  func(*Conn, error)

	outbound chan types.Message

	writeMu sync.Mutex

	closeOnce sync.Once
	closed    atomic.Bool

	terminate chan struct{}

	closeErr atomic.Pointer[error]

	pongCh chan struct{}

	closeCode atomic.Int32
}

// Options bundles the per-conn wiring supplied by the upgrade flow.
type Options struct {
	WS        *gws.Conn
	Cfg       types.Config
	Principal types.Principal
	RemoteIP  string
	Subproto  string
	Metrics   types.MetricsRecorder
	OnClose   func(*Conn, error)
}

// New wraps an upgraded gorilla conn. The caller starts the conn's
// goroutine by invoking Run.
func New(opt Options) *Conn {
	c := &Conn{
		id:        NewID(),
		principal: opt.Principal,
		remoteIP:  opt.RemoteIP,
		subproto:  opt.Subproto,
		ws:        opt.WS,
		cfg:       opt.Cfg,
		metrics:   opt.Metrics,
		onClose:   opt.OnClose,
		outbound:  make(chan types.Message, opt.Cfg.WriteBuffer),
		terminate: make(chan struct{}),
		pongCh:    make(chan struct{}, 1),
	}
	opt.WS.SetReadLimit(opt.Cfg.ReadLimit)
	return c
}

func (c *Conn) ID() string                { return c.id }
func (c *Conn) Principal() types.Principal { return c.principal }
func (c *Conn) RemoteIP() string          { return c.remoteIP }
func (c *Conn) Subprotocol() string       { return c.subproto }

// LastCloseCode returns the close code emitted by this side, or
// CloseNormalClosure if none has been recorded.
func (c *Conn) LastCloseCode() int {
	v := c.closeCode.Load()
	if v == 0 {
		return types.CloseNormalClosure
	}
	return int(v)
}

// Send encodes v through the configured codec and enqueues the result.
func (c *Conn) Send(v interface{}) error {
	if c.closed.Load() {
		return types.ErrConnClosed
	}
	m, err := c.cfg.Codec.Encode(v)
	if err != nil {
		return err
	}
	return c.sendMessage(m)
}

// SendMessage enqueues an already-encoded Message.
func (c *Conn) SendMessage(m types.Message) error {
	return c.sendMessage(m)
}

func (c *Conn) sendMessage(m types.Message) error {
	if c.closed.Load() {
		return types.ErrConnClosed
	}
	select {
	case c.outbound <- m:
		return nil
	default:
		return types.ErrWriteQueueFull
	}
}

// Close terminates the connection with the given application close code
// and reason. Idempotent.
func (c *Conn) Close(code int, reason string) error {
	var firstErr error
	c.closeOnce.Do(func() {
		c.closeCode.Store(int32(code))
		c.closed.Store(true)
		firstErr = c.writeClose(code, reason)
		_ = c.ws.Close()
		close(c.terminate)
	})
	return firstErr
}

func (c *Conn) writeClose(code int, reason string) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.ws.WriteControl(gws.CloseMessage, gws.FormatCloseMessage(code, reason), time.Now().Add(c.cfg.HandshakeTimeout))
}

// Run drives the read/write/heartbeat goroutines and blocks until they
// all exit. Called by the upgrade flow in its own goroutine.
func (c *Conn) Run(ctx context.Context) {
	if cb := c.cfg.OnConnect; cb != nil {
		if err := cb(c); err != nil {
			c.recordClose(err)
			_ = c.Close(types.CloseInternalServerErr, "OnConnect rejected")
			return
		}
	}

	c.ws.SetReadDeadline(time.Now().Add(c.cfg.PongTimeout))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(c.cfg.PongTimeout))
		select {
		case c.pongCh <- struct{}{}:
		default:
		}
		return nil
	})

	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); c.writePump(ctx) }()
	go func() { defer wg.Done(); c.readPump(ctx) }()
	go func() { defer wg.Done(); c.heartbeat() }()
	wg.Wait()

	if cb := c.cfg.OnClose; cb != nil {
		var err error
		if ep := c.closeErr.Load(); ep != nil {
			err = *ep
		}
		cb(c, err)
	}
	if c.onClose != nil {
		var err error
		if ep := c.closeErr.Load(); ep != nil {
			err = *ep
		}
		c.onClose(c, err)
	}
}

func (c *Conn) readPump(ctx context.Context) {
	defer c.Close(types.CloseNormalClosure, "")

	for {
		if ctx.Err() != nil {
			c.recordClose(ctx.Err())
			return
		}
		mt, data, err := c.ws.ReadMessage()
		if err != nil {
			c.recordClose(err)
			return
		}

		if int64(len(data)) > c.cfg.MessageReassemblyLimit {
			c.recordClose(types.ErrMessageTooLarge)
			_ = c.Close(types.CloseMessageTooBig, "message too large")
			return
		}

		if c.cfg.RateLimit != nil && !c.cfg.RateLimit.Allow(len(data)) {
			c.recordClose(types.ErrLimitExceeded)
			_ = c.Close(types.CloseRateLimitExceeded, "rate limit exceeded")
			return
		}

		if types.MessageType(mt) == types.TextMessage && !ValidUTF8(data) {
			c.recordClose(types.ErrInvalidUTF8)
			_ = c.Close(types.CloseInvalidFramePayload, "invalid utf-8")
			return
		}

		msg := types.Message{Type: types.MessageType(mt), Data: data}
		if c.metrics != nil {
			c.metrics.RecordMessageIn(len(data))
		}

		if cb := c.cfg.OnMessage; cb != nil {
			if err := cb(c, msg); err != nil {
				c.recordClose(err)
				_ = c.Close(types.CloseInternalServerErr, "")
				return
			}
		}
	}
}

func (c *Conn) writePump(ctx context.Context) {
	defer c.Close(types.CloseNormalClosure, "")

	for {
		select {
		case <-ctx.Done():
			c.recordClose(ctx.Err())
			return
		case <-c.terminate:
			return
		case m, ok := <-c.outbound:
			if !ok {
				return
			}
			if err := c.writeOne(m); err != nil {
				c.recordClose(err)
				return
			}
		}
	}
}

func (c *Conn) writeOne(m types.Message) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.ws.SetWriteDeadline(time.Now().Add(c.cfg.HandshakeTimeout)); err != nil {
		return err
	}
	if err := c.ws.WriteMessage(int(m.Type), m.Data); err != nil {
		return err
	}
	if c.metrics != nil {
		c.metrics.RecordMessageOut(len(m.Data))
	}
	return nil
}

func (c *Conn) recordClose(err error) {
	if err == nil {
		return
	}
	if gws.IsCloseError(err, gws.CloseNormalClosure, gws.CloseGoingAway) ||
		errors.Is(err, net.ErrClosed) {
		return
	}
	if c.closeErr.Load() == nil {
		c.closeErr.Store(&err)
	}
}

// NewID returns a random 128-bit hex identifier.
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("websocket/conn: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}
