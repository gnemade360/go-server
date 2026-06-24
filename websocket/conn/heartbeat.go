package conn

import (
	"time"

	gws "github.com/gorilla/websocket"

	"github.com/gnemade360/go-server/websocket/internal/types"
)

func (c *Conn) heartbeat() {
	if c.cfg.PingInterval <= 0 {
		return
	}
	ticker := time.NewTicker(c.cfg.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.terminate:
			return
		case <-ticker.C:
			if err := c.sendPing(); err != nil {
				c.recordClose(err)
				_ = c.Close(types.CloseInternalServerErr, "ping failed")
				return
			}
		}
	}
}

func (c *Conn) sendPing() error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.ws.WriteControl(gws.PingMessage, nil, time.Now().Add(c.cfg.HandshakeTimeout))
}
