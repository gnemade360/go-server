package websocket

import (
	"context"
	"time"
)

// timeoutCtx returns a context that expires after d. Test helper
// so individual tests stay terse.
func timeoutCtx(d time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	// Cancel runs when ctx expires; the test never holds the
	// cancel func because each call site uses the context once.
	_ = cancel
	return ctx
}
