// Package types defines the shared vocabulary used by every websocket
// subpackage. It is in internal/ so user code cannot import it directly;
// users only see the re-exported names from the root websocket package.
package types

import (
	"errors"
	"log"
	"net/http"
	"time"
)

const (
	CloseNormalClosure       = 1000
	CloseGoingAway           = 1001
	CloseProtocolError       = 1002
	CloseUnsupportedData     = 1003
	CloseInvalidFramePayload = 1007
	ClosePolicyViolation     = 1008
	CloseMessageTooBig       = 1009
	CloseInternalServerErr   = 1011
	CloseServiceRestart      = 1012
	CloseTryAgainLater       = 1013

	CloseRateLimitExceeded = 4008
	CloseQuotaExceeded     = 4029
)

var (
	ErrHandlerClosed   = errors.New("websocket: handler is shutting down")
	ErrOriginRejected  = errors.New("websocket: origin rejected")
	ErrUnauthorized    = errors.New("websocket: unauthorized")
	ErrLimitExceeded   = errors.New("websocket: connection limit exceeded")
	ErrSubprotocol     = errors.New("websocket: no acceptable subprotocol")
	ErrWriteQueueFull  = errors.New("websocket: write queue full")
	ErrMessageTooLarge = errors.New("websocket: message exceeds size limit")
	ErrInvalidUTF8     = errors.New("websocket: invalid UTF-8 in text frame")
	ErrConnClosed      = errors.New("websocket: connection closed")
)

// MessageType matches gorilla/websocket frame type constants.
type MessageType int

const (
	TextMessage   MessageType = 1
	BinaryMessage MessageType = 2
)

// Message is an inbound or outbound application message.
type Message struct {
	Type MessageType
	Data []byte
}

// Principal is the identity attached to an open connection.
type Principal struct {
	ID         string
	Roles      []string
	Attributes map[string]interface{}
}

// OriginChecker decides whether an upgrade request's Origin is acceptable.
type OriginChecker interface {
	CheckOrigin(r *http.Request) bool
}

// Authenticator authenticates an upgrade request.
type Authenticator interface {
	Authenticate(r *http.Request) (Principal, error)
}

// ConnectionLimiter caps concurrent connections.
type ConnectionLimiter interface {
	Acquire(r *http.Request, p Principal) (release func(), err error)
}

// CapInspector is an optional interface a ConnectionLimiter may
// implement so health checks can report the global cap.
type CapInspector interface {
	GlobalCap() int
}

// MessageCodec encodes outbound values and decodes inbound bytes.
type MessageCodec interface {
	Encode(v interface{}) (Message, error)
	Decode(m Message, dst interface{}) error
}

// MessageRateLimiter caps how often and how many bytes one connection
// may send.
type MessageRateLimiter interface {
	Allow(n int) bool
}

// Config configures a Handler. The zero value is safe; the root
// package's Config.WithDefaults fills unset fields.
type Config struct {
	Origin       OriginChecker
	Auth         Authenticator
	Limits       ConnectionLimiter
	Subprotocols []string
	Codec        MessageCodec
	RateLimit    MessageRateLimiter

	ReadLimit              int64
	MessageReassemblyLimit int64
	WriteBuffer            int
	PingInterval           time.Duration
	PongTimeout            time.Duration
	HandshakeTimeout       time.Duration
	EnableCompression      bool

	Logger *log.Logger

	OnConnect func(ConnRef) error
	OnMessage func(ConnRef, Message) error
	OnClose   func(ConnRef, error)
}

// ConnRef is the contract callbacks see for a connection. It is an
// interface (not a concrete type) so the conn package can implement
// it without creating an import cycle back to types.
type ConnRef interface {
	ID() string
	Principal() Principal
	RemoteIP() string
	Subprotocol() string
	Send(v interface{}) error
	SendMessage(m Message) error
	Close(code int, reason string) error
}

// MetricsRecorder is the optional interface the conn package uses to
// emit per-frame metrics. The ops package implements it; an empty
// (no-op) implementation is also fine.
type MetricsRecorder interface {
	RecordMessageIn(size int)
	RecordMessageOut(size int)
}

// HubRegistrar is the optional interface for hub registration calls
// made from the upgrade flow. The hub package implements it.
type HubRegistrar interface {
	Register(ConnRef)
	Unregister(ConnRef)
}
