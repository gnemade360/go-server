package health

import (
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"time"
)

// MemoryCheck creates a memory usage health check
func MemoryCheck(maxMemoryMB int) CheckFunc {
	return func() *Check {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		currentMemoryMB := int(m.Alloc / 1024 / 1024)
		
		status := StatusUp
		message := fmt.Sprintf("Memory usage: %d MB", currentMemoryMB)
		
		if maxMemoryMB > 0 && currentMemoryMB > maxMemoryMB {
			status = StatusDown
			message = fmt.Sprintf("Memory usage too high: %d MB (max: %d MB)", currentMemoryMB, maxMemoryMB)
		} else if maxMemoryMB > 0 && currentMemoryMB > int(float64(maxMemoryMB)*0.8) {
			status = StatusWarning
			message = fmt.Sprintf("Memory usage high: %d MB (max: %d MB)", currentMemoryMB, maxMemoryMB)
		}
		
		return &Check{
			Status:  status,
			Message: message,
			Details: map[string]interface{}{
				"alloc_mb":      currentMemoryMB,
				"sys_mb":        int(m.Sys / 1024 / 1024),
				"gc_runs":       m.NumGC,
				"goroutines":    runtime.NumGoroutine(),
				"max_memory_mb": maxMemoryMB,
			},
		}
	}
}

// GoroutineCheck creates a goroutine count health check
func GoroutineCheck(maxGoroutines int) CheckFunc {
	return func() *Check {
		count := runtime.NumGoroutine()
		
		status := StatusUp
		message := fmt.Sprintf("Goroutines: %d", count)
		
		if maxGoroutines > 0 && count > maxGoroutines {
			status = StatusDown
			message = fmt.Sprintf("Too many goroutines: %d (max: %d)", count, maxGoroutines)
		} else if maxGoroutines > 0 && count > int(float64(maxGoroutines)*0.8) {
			status = StatusWarning
			message = fmt.Sprintf("High goroutine count: %d (max: %d)", count, maxGoroutines)
		}
		
		return &Check{
			Status:  status,
			Message: message,
			Details: map[string]interface{}{
				"count": count,
				"max":   maxGoroutines,
			},
		}
	}
}

// HTTPCheck creates an HTTP endpoint health check
func HTTPCheck(name, url string, timeout time.Duration) CheckFunc {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	
	return func() *Check {
		client := &http.Client{Timeout: timeout}
		start := time.Now()
		
		resp, err := client.Get(url)
		duration := time.Since(start)
		
		if err != nil {
			return &Check{
				Status:  StatusDown,
				Message: fmt.Sprintf("HTTP check failed: %v", err),
				Details: map[string]interface{}{
					"url":      url,
					"error":    err.Error(),
					"duration": duration.String(),
				},
			}
		}
		defer resp.Body.Close()
		
		status := StatusUp
		message := fmt.Sprintf("HTTP check successful: %d", resp.StatusCode)
		
		if resp.StatusCode >= 500 {
			status = StatusDown
			message = fmt.Sprintf("HTTP check failed: %d", resp.StatusCode)
		} else if resp.StatusCode >= 400 {
			status = StatusWarning
			message = fmt.Sprintf("HTTP check warning: %d", resp.StatusCode)
		}
		
		return &Check{
			Status:  status,
			Message: message,
			Details: map[string]interface{}{
				"url":         url,
				"status_code": resp.StatusCode,
				"duration":    duration.String(),
			},
		}
	}
}

// DiskSpaceCheck creates a disk space health check
func DiskSpaceCheck(path string, minFreeGB int) CheckFunc {
	return func() *Check {
		// Note: This is a simplified version. In a real implementation,
		// you would use syscalls to get actual disk space information
		// For now, we'll create a basic check
		
		status := StatusUp
		message := "Disk space check not fully implemented"
		
		return &Check{
			Status:  status,
			Message: message,
			Details: map[string]interface{}{
				"path":        path,
				"min_free_gb": minFreeGB,
				"note":        "Disk space monitoring requires platform-specific implementation",
			},
		}
	}
}

// DatabaseCheck creates a database connectivity health check
func DatabaseCheck(name string, pingFunc func() error) CheckFunc {
	return func() *Check {
		err := pingFunc()
		
		if err != nil {
			return &Check{
				Status:  StatusDown,
				Message: fmt.Sprintf("Database %s unreachable: %v", name, err),
				Details: map[string]interface{}{
					"database": name,
					"error":    err.Error(),
				},
			}
		}
		
		return &Check{
			Status:  StatusUp,
			Message: fmt.Sprintf("Database %s is healthy", name),
			Details: map[string]interface{}{
				"database": name,
			},
		}
	}
}

// CustomCheck creates a custom health check with a user-defined function
func CustomCheck(name string, checkFunc func() (Status, string, map[string]interface{})) CheckFunc {
	return func() *Check {
		status, message, details := checkFunc()
		
		return &Check{
			Status:  status,
			Message: message,
			Details: details,
		}
	}
}

// URLReachabilityCheck creates a basic URL reachability check
func URLReachabilityCheck(targetURL string, timeout time.Duration) CheckFunc {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	
	return func() *Check {
		// Parse URL to validate it
		parsedURL, err := url.Parse(targetURL)
		if err != nil {
			return &Check{
				Status:  StatusDown,
				Message: fmt.Sprintf("Invalid URL: %v", err),
				Details: map[string]interface{}{
					"url":   targetURL,
					"error": err.Error(),
				},
			}
		}
		
		client := &http.Client{Timeout: timeout}
		start := time.Now()
		
		// Make HEAD request to check reachability without downloading content
		req, err := http.NewRequest("HEAD", targetURL, nil)
		if err != nil {
			return &Check{
				Status:  StatusDown,
				Message: fmt.Sprintf("Failed to create request: %v", err),
				Details: map[string]interface{}{
					"url":   targetURL,
					"error": err.Error(),
				},
			}
		}
		
		resp, err := client.Do(req)
		duration := time.Since(start)
		
		if err != nil {
			return &Check{
				Status:  StatusDown,
				Message: fmt.Sprintf("URL unreachable: %v", err),
				Details: map[string]interface{}{
					"url":      targetURL,
					"host":     parsedURL.Host,
					"error":    err.Error(),
					"duration": duration.String(),
				},
			}
		}
		defer resp.Body.Close()
		
		return &Check{
			Status:  StatusUp,
			Message: fmt.Sprintf("URL reachable: %s", targetURL),
			Details: map[string]interface{}{
				"url":         targetURL,
				"host":        parsedURL.Host,
				"status_code": resp.StatusCode,
				"duration":    duration.String(),
			},
		}
	}
}