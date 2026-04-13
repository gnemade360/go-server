package health

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewHealthChecker(t *testing.T) {
	hc := NewHealthChecker()
	
	if hc == nil {
		t.Fatal("NewHealthChecker returned nil")
	}
	
	if hc.checks == nil {
		t.Error("checks map not initialized")
	}
	
	if hc.serviceInfo == nil {
		t.Error("serviceInfo map not initialized")
	}
	
	if hc.timeout != 5*time.Second {
		t.Errorf("Expected default timeout 5s, got %v", hc.timeout)
	}
}

func TestHealthChecker_AddCheck(t *testing.T) {
	hc := NewHealthChecker()
	
	checkFunc := func() *Check {
		return &Check{Status: StatusUp, Message: "test"}
	}
	
	hc.AddCheck("test-check", checkFunc)
	
	hc.mu.RLock()
	if len(hc.checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(hc.checks))
	}
	
	if _, exists := hc.checks["test-check"]; !exists {
		t.Error("Check was not added")
	}
	hc.mu.RUnlock()
}

func TestHealthChecker_RemoveCheck(t *testing.T) {
	hc := NewHealthChecker()
	
	checkFunc := func() *Check {
		return &Check{Status: StatusUp, Message: "test"}
	}
	
	hc.AddCheck("test-check", checkFunc)
	hc.RemoveCheck("test-check")
	
	hc.mu.RLock()
	if len(hc.checks) != 0 {
		t.Errorf("Expected 0 checks, got %d", len(hc.checks))
	}
	hc.mu.RUnlock()
}

func TestHealthChecker_SetServiceInfo(t *testing.T) {
	hc := NewHealthChecker()
	
	hc.SetServiceInfo("version", "1.0.0")
	hc.SetServiceInfo("environment", "test")
	
	hc.mu.RLock()
	if len(hc.serviceInfo) != 2 {
		t.Errorf("Expected 2 service info entries, got %d", len(hc.serviceInfo))
	}
	
	if hc.serviceInfo["version"] != "1.0.0" {
		t.Error("Version not set correctly")
	}
	
	if hc.serviceInfo["environment"] != "test" {
		t.Error("Environment not set correctly")
	}
	hc.mu.RUnlock()
}

func TestHealthChecker_SetTimeout(t *testing.T) {
	hc := NewHealthChecker()
	
	timeout := 10 * time.Second
	hc.SetTimeout(timeout)
	
	hc.mu.RLock()
	if hc.timeout != timeout {
		t.Errorf("Expected timeout %v, got %v", timeout, hc.timeout)
	}
	hc.mu.RUnlock()
}

func TestHealthChecker_GetHealth_NoChecks(t *testing.T) {
	hc := NewHealthChecker()
	
	health := hc.GetHealth()
	
	if health == nil {
		t.Fatal("GetHealth returned nil")
	}
	
	if health.Status != StatusUp {
		t.Errorf("Expected status UP, got %s", health.Status)
	}
	
	if len(health.Checks) != 0 {
		t.Errorf("Expected 0 checks, got %d", len(health.Checks))
	}
	
	if health.Timestamp.IsZero() {
		t.Error("Timestamp not set")
	}
}

func TestHealthChecker_GetHealth_WithChecks(t *testing.T) {
	hc := NewHealthChecker()
	
	upCheck := func() *Check {
		return &Check{Status: StatusUp, Message: "up check"}
	}
	
	downCheck := func() *Check {
		return &Check{Status: StatusDown, Message: "down check"}
	}
	
	hc.AddCheck("up-check", upCheck)
	hc.AddCheck("down-check", downCheck)
	
	health := hc.GetHealth()
	
	if health.Status != StatusDown {
		t.Errorf("Expected overall status DOWN, got %s", health.Status)
	}
	
	if len(health.Checks) != 2 {
		t.Errorf("Expected 2 checks, got %d", len(health.Checks))
	}
	
	upResult := health.Checks["up-check"]
	if upResult == nil || upResult.Status != StatusUp {
		t.Error("Up check result incorrect")
	}
	
	downResult := health.Checks["down-check"]
	if downResult == nil || downResult.Status != StatusDown {
		t.Error("Down check result incorrect")
	}
}

func TestHealthChecker_GetHealth_WithWarning(t *testing.T) {
	hc := NewHealthChecker()
	
	upCheck := func() *Check {
		return &Check{Status: StatusUp, Message: "up check"}
	}
	
	warningCheck := func() *Check {
		return &Check{Status: StatusWarning, Message: "warning check"}
	}
	
	hc.AddCheck("up-check", upCheck)
	hc.AddCheck("warning-check", warningCheck)
	
	health := hc.GetHealth()
	
	if health.Status != StatusWarning {
		t.Errorf("Expected overall status WARNING, got %s", health.Status)
	}
}

func TestHealthChecker_GetHealth_Timeout(t *testing.T) {
	hc := NewHealthChecker()
	hc.SetTimeout(100 * time.Millisecond)
	
	slowCheck := func() *Check {
		time.Sleep(200 * time.Millisecond)
		return &Check{Status: StatusUp, Message: "slow check"}
	}
	
	hc.AddCheck("slow-check", slowCheck)
	
	health := hc.GetHealth()
	
	if health.Status != StatusDown {
		t.Errorf("Expected overall status DOWN due to timeout, got %s", health.Status)
	}
	
	slowResult := health.Checks["slow-check"]
	if slowResult == nil || slowResult.Status != StatusDown {
		t.Error("Slow check should have timed out")
	}
	
	if slowResult.Message != "Health check timed out" {
		t.Errorf("Expected timeout message, got %s", slowResult.Message)
	}
}

func TestHealthChecker_GetHealth_Panic(t *testing.T) {
	hc := NewHealthChecker()
	
	panicCheck := func() *Check {
		panic("test panic")
	}
	
	hc.AddCheck("panic-check", panicCheck)
	
	health := hc.GetHealth()
	
	if health.Status != StatusDown {
		t.Errorf("Expected overall status DOWN due to panic, got %s", health.Status)
	}
	
	panicResult := health.Checks["panic-check"]
	if panicResult == nil || panicResult.Status != StatusDown {
		t.Error("Panic check should have failed")
	}
	
	if panicResult.Message != "Health check panicked" {
		t.Errorf("Expected panic message, got %s", panicResult.Message)
	}
}

func TestHealthChecker_GetHealth_NilCheck(t *testing.T) {
	hc := NewHealthChecker()
	
	nilCheck := func() *Check {
		return nil
	}
	
	hc.AddCheck("nil-check", nilCheck)
	
	health := hc.GetHealth()
	
	if health.Status != StatusDown {
		t.Errorf("Expected overall status DOWN due to nil check, got %s", health.Status)
	}
	
	nilResult := health.Checks["nil-check"]
	if nilResult == nil || nilResult.Status != StatusDown {
		t.Error("Nil check should have failed")
	}
	
	if nilResult.Message != "Health check returned nil" {
		t.Errorf("Expected nil check message, got %s", nilResult.Message)
	}
}

func TestHealthChecker_GetLastHealth(t *testing.T) {
	hc := NewHealthChecker()
	
	// Initially should return nil
	lastHealth := hc.GetLastHealth()
	if lastHealth != nil {
		t.Error("Expected nil for initial GetLastHealth")
	}
	
	// After running GetHealth, should return the result
	health := hc.GetHealth()
	lastHealth = hc.GetLastHealth()
	
	if lastHealth != health {
		t.Error("GetLastHealth should return same result as GetHealth")
	}
}

func TestHealthChecker_Handler(t *testing.T) {
	hc := NewHealthChecker()
	hc.SetServiceInfo("version", "1.0.0")
	
	upCheck := func() *Check {
		return &Check{Status: StatusUp, Message: "test check"}
	}
	hc.AddCheck("test-check", upCheck)
	
	handler := hc.Handler()
	
	// Test GET request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	
	handler(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
	
	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	
	if response.Status != StatusUp {
		t.Errorf("Expected status UP, got %s", response.Status)
	}
	
	if len(response.Checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(response.Checks))
	}
	
	if response.ServiceInfo["version"] != "1.0.0" {
		t.Error("Service info not included")
	}
}

func TestHealthChecker_Handler_DownStatus(t *testing.T) {
	hc := NewHealthChecker()
	
	downCheck := func() *Check {
		return &Check{Status: StatusDown, Message: "service down"}
	}
	hc.AddCheck("down-check", downCheck)
	
	handler := hc.Handler()
	
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	
	handler(w, req)
	
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestHealthChecker_Handler_MethodNotAllowed(t *testing.T) {
	hc := NewHealthChecker()
	handler := hc.Handler()
	
	req := httptest.NewRequest("POST", "/health", nil)
	w := httptest.NewRecorder()
	
	handler(w, req)
	
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHealthChecker_ReadinessHandler(t *testing.T) {
	hc := NewHealthChecker()
	
	upCheck := func() *Check {
		return &Check{Status: StatusUp, Message: "ready"}
	}
	hc.AddCheck("ready-check", upCheck)
	
	handler := hc.ReadinessHandler()
	
	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()
	
	handler(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	body := w.Body.String()
	if body != "READY" {
		t.Errorf("Expected 'READY', got %s", body)
	}
}

func TestHealthChecker_ReadinessHandler_NotReady(t *testing.T) {
	hc := NewHealthChecker()
	
	downCheck := func() *Check {
		return &Check{Status: StatusDown, Message: "not ready"}
	}
	hc.AddCheck("not-ready-check", downCheck)
	
	handler := hc.ReadinessHandler()
	
	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()
	
	handler(w, req)
	
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
	
	body := w.Body.String()
	if body != "NOT READY" {
		t.Errorf("Expected 'NOT READY', got %s", body)
	}
}

func TestLivenessHandler(t *testing.T) {
	handler := LivenessHandler()
	
	req := httptest.NewRequest("GET", "/live", nil)
	w := httptest.NewRecorder()
	
	handler(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	body := w.Body.String()
	if body != "ALIVE" {
		t.Errorf("Expected 'ALIVE', got %s", body)
	}
}

func TestLivenessHandler_MethodNotAllowed(t *testing.T) {
	handler := LivenessHandler()
	
	req := httptest.NewRequest("POST", "/live", nil)
	w := httptest.NewRecorder()
	
	handler(w, req)
	
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func BenchmarkHealthChecker_GetHealth(b *testing.B) {
	hc := NewHealthChecker()
	
	// Add multiple checks
	for i := 0; i < 5; i++ {
		hc.AddCheck(fmt.Sprintf("check-%d", i), func() *Check {
			return &Check{Status: StatusUp, Message: "benchmark check"}
		})
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hc.GetHealth()
	}
}

func BenchmarkHealthChecker_Handler(b *testing.B) {
	hc := NewHealthChecker()
	hc.AddCheck("benchmark-check", func() *Check {
		return &Check{Status: StatusUp, Message: "benchmark check"}
	})
	
	handler := hc.Handler()
	req := httptest.NewRequest("GET", "/health", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler(w, req)
	}
}