package health

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status of a component
type Status string

const (
	StatusUp      Status = "UP"
	StatusDown    Status = "DOWN"
	StatusWarning Status = "WARNING"
)

// Check represents a single health check
type Check struct {
	Name        string                 `json:"name"`
	Status      Status                 `json:"status"`
	Message     string                 `json:"message,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	LastChecked time.Time              `json:"last_checked"`
	Duration    time.Duration          `json:"duration"`
}

// HealthResponse represents the overall health response
type HealthResponse struct {
	Status      Status            `json:"status"`
	Timestamp   time.Time         `json:"timestamp"`
	Duration    time.Duration     `json:"duration"`
	Checks      map[string]*Check `json:"checks"`
	ServiceInfo map[string]string `json:"service_info,omitempty"`
}

// CheckFunc is a function that performs a health check
type CheckFunc func() *Check

// HealthChecker manages health checks
type HealthChecker struct {
	mu           sync.RWMutex
	checks       map[string]CheckFunc
	serviceInfo  map[string]string
	timeout      time.Duration
	lastResponse *HealthResponse
}

// NewHealthChecker creates a new health checker
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checks:      make(map[string]CheckFunc),
		serviceInfo: make(map[string]string),
		timeout:     5 * time.Second,
	}
}

// AddCheck adds a health check
func (hc *HealthChecker) AddCheck(name string, check CheckFunc) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks[name] = check
}

// RemoveCheck removes a health check
func (hc *HealthChecker) RemoveCheck(name string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	delete(hc.checks, name)
}

// SetServiceInfo sets service information
func (hc *HealthChecker) SetServiceInfo(key, value string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.serviceInfo[key] = value
}

// SetTimeout sets the timeout for health checks
func (hc *HealthChecker) SetTimeout(timeout time.Duration) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.timeout = timeout
}

// runCheck executes a single health check with timeout
func (hc *HealthChecker) runCheck(name string, checkFn CheckFunc) *Check {
	start := time.Now()
	done := make(chan *Check, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- &Check{
					Name:        name,
					Status:      StatusDown,
					Message:     "Health check panicked",
					Details:     map[string]interface{}{"panic": r},
					LastChecked: start,
					Duration:    time.Since(start),
				}
			}
		}()
		
		check := checkFn()
		if check == nil {
			check = &Check{
				Name:   name,
				Status: StatusDown,
				Message: "Health check returned nil",
			}
		}
		check.Name = name
		check.LastChecked = start
		check.Duration = time.Since(start)
		done <- check
	}()

	select {
	case check := <-done:
		return check
	case <-time.After(hc.timeout):
		return &Check{
			Name:        name,
			Status:      StatusDown,
			Message:     "Health check timed out",
			Details:     map[string]interface{}{"timeout": hc.timeout.String()},
			LastChecked: start,
			Duration:    hc.timeout,
		}
	}
}

// GetHealth performs all health checks and returns the result
func (hc *HealthChecker) GetHealth() *HealthResponse {
	start := time.Now()
	
	hc.mu.RLock()
	checks := make(map[string]CheckFunc, len(hc.checks))
	for name, check := range hc.checks {
		checks[name] = check
	}
	serviceInfo := make(map[string]string, len(hc.serviceInfo))
	for k, v := range hc.serviceInfo {
		serviceInfo[k] = v
	}
	hc.mu.RUnlock()

	checkResults := make(map[string]*Check)
	overallStatus := StatusUp

	for name, checkFn := range checks {
		check := hc.runCheck(name, checkFn)
		checkResults[name] = check

		if check.Status == StatusDown {
			overallStatus = StatusDown
		} else if check.Status == StatusWarning && overallStatus == StatusUp {
			overallStatus = StatusWarning
		}
	}

	response := &HealthResponse{
		Status:      overallStatus,
		Timestamp:   start,
		Duration:    time.Since(start),
		Checks:      checkResults,
		ServiceInfo: serviceInfo,
	}

	hc.mu.Lock()
	hc.lastResponse = response
	hc.mu.Unlock()

	return response
}

// GetLastHealth returns the last health check result without running checks
func (hc *HealthChecker) GetLastHealth() *HealthResponse {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.lastResponse
}

// Handler returns an HTTP handler for health checks
func (hc *HealthChecker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		health := hc.GetHealth()
		
		w.Header().Set("Content-Type", "application/json")
		
		// Set appropriate HTTP status based on health
		switch health.Status {
		case StatusUp:
			w.WriteHeader(http.StatusOK)
		case StatusWarning:
			w.WriteHeader(http.StatusOK) // 200 for warnings
		case StatusDown:
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		if err := json.NewEncoder(w).Encode(health); err != nil {
			http.Error(w, "Failed to encode health response", http.StatusInternalServerError)
		}
	}
}

// ReadinessHandler returns a simple readiness check handler
func (hc *HealthChecker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		health := hc.GetHealth()
		
		if health.Status == StatusDown {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("NOT READY"))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("READY"))
	}
}

// LivenessHandler returns a simple liveness check handler
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ALIVE"))
	}
}