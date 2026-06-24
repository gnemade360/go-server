package ops

import (
	"fmt"

	"github.com/gnemade360/go-server/health"
	"github.com/gnemade360/go-server/websocket/internal/types"
)

// HealthSnapshot is the data a Handler exposes to the health check.
type HealthSnapshot struct {
	ShuttingDown bool
	HubRunning   bool   // false when no hub was created
	HubStopped   bool   // true only if a hub was created and has since stopped
	Active       int
	Limiter      types.ConnectionLimiter
}

// Check returns a health.CheckFunc that reports the WebSocket
// subsystem state by calling snapshot. The snapshot function captures
// state lazily so the check sees the most recent values at each
// invocation.
func Check(snapshot func() HealthSnapshot) health.CheckFunc {
	return func() *health.Check {
		s := snapshot()
		if s.ShuttingDown {
			return &health.Check{
				Name:    "websocket",
				Status:  health.StatusDown,
				Message: "handler is shutting down",
			}
		}
		if s.HubStopped {
			return &health.Check{
				Name:    "websocket",
				Status:  health.StatusDown,
				Message: "hub goroutine is not running",
			}
		}

		globalCap := 0
		if insp, ok := s.Limiter.(types.CapInspector); ok {
			globalCap = insp.GlobalCap()
		}

		details := map[string]interface{}{
			"active":     s.Active,
			"global_cap": globalCap,
		}

		status := health.StatusUp
		message := "ok"
		if globalCap > 0 && float64(s.Active)/float64(globalCap) > 0.8 {
			status = health.StatusWarning
			message = fmt.Sprintf("active connections at %d/%d (>80%%)", s.Active, globalCap)
		}

		return &health.Check{
			Name:    "websocket",
			Status:  status,
			Message: message,
			Details: details,
		}
	}
}
