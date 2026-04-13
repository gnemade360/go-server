package health

import (
	"time"
)

// ServerHealthChecker provides a pre-configured health checker for the go-server
type ServerHealthChecker struct {
	*HealthChecker
}

// NewServerHealthChecker creates a new health checker with common server checks
func NewServerHealthChecker() *ServerHealthChecker {
	hc := NewHealthChecker()
	
	// Set default service information
	hc.SetServiceInfo("service", "go-server")
	hc.SetServiceInfo("version", "1.0.0")
	
	// Add basic system checks
	hc.AddCheck("memory", MemoryCheck(512)) // 512 MB limit
	hc.AddCheck("goroutines", GoroutineCheck(1000)) // 1000 goroutine limit
	
	return &ServerHealthChecker{
		HealthChecker: hc,
	}
}

// AddDatabaseCheck adds a database connectivity check
func (shc *ServerHealthChecker) AddDatabaseCheck(name string, pingFunc func() error) {
	shc.AddCheck("database-"+name, DatabaseCheck(name, pingFunc))
}

// AddExternalServiceCheck adds an external service dependency check
func (shc *ServerHealthChecker) AddExternalServiceCheck(name, url string) {
	shc.AddCheck("external-"+name, HTTPCheck(name, url, 10*time.Second))
}

// AddCustomCheck adds a custom health check with a descriptive name
func (shc *ServerHealthChecker) AddCustomCheck(name string, checkFunc func() (Status, string, map[string]interface{})) {
	shc.AddCheck(name, CustomCheck(name, checkFunc))
}

// SetVersion updates the service version information
func (shc *ServerHealthChecker) SetVersion(version string) {
	shc.SetServiceInfo("version", version)
}

// SetEnvironment sets the environment information
func (shc *ServerHealthChecker) SetEnvironment(env string) {
	shc.SetServiceInfo("environment", env)
}

// EnableDetailedChecks adds more detailed system monitoring
func (shc *ServerHealthChecker) EnableDetailedChecks() {
	// Add disk space check for common directories
	shc.AddCheck("disk-tmp", DiskSpaceCheck("/tmp", 1)) // 1 GB minimum
	
	// Add a check for basic URL reachability (can be used for DNS resolution)
	shc.AddCheck("dns-resolution", URLReachabilityCheck("https://www.google.com", 5*time.Second))
}

// ConfigureForProduction sets up health checks optimized for production use
func (shc *ServerHealthChecker) ConfigureForProduction() {
	// Set shorter timeout for production
	shc.SetTimeout(3 * time.Second)
	
	// Add production-specific checks
	shc.AddCheck("memory-production", MemoryCheck(1024)) // 1GB limit for production
	shc.AddCheck("goroutines-production", GoroutineCheck(500)) // Lower limit for production
	
	// Enable detailed monitoring
	shc.EnableDetailedChecks()
}

// ConfigureForDevelopment sets up health checks optimized for development
func (shc *ServerHealthChecker) ConfigureForDevelopment() {
	// Set longer timeout for development
	shc.SetTimeout(10 * time.Second)
	
	// Add development-specific information
	shc.SetEnvironment("development")
	
	// More relaxed limits for development
	shc.AddCheck("memory-dev", MemoryCheck(2048)) // 2GB limit for development
	shc.AddCheck("goroutines-dev", GoroutineCheck(2000)) // Higher limit for development
}