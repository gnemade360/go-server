# Health Monitoring Example

This example demonstrates how to use the health monitoring system in the go-server framework.

## Features Demonstrated

- **Basic Health Checks**: Memory usage, goroutine count
- **Database Health Checks**: Simulated database connectivity monitoring
- **External Service Checks**: HTTP-based dependency monitoring
- **Custom Application Checks**: Business logic and performance monitoring
- **Multiple Health Endpoints**: Full health, readiness, and liveness checks

## Running the Example

```bash
cd examples/health-monitoring
go run main.go
```

The server will start on port 8080.

## Health Endpoints

### Full Health Check
- **URL**: `GET /health`
- **Purpose**: Comprehensive health status including all checks
- **Response**: JSON with detailed health information
- **HTTP Status**: 
  - `200 OK`: All checks passing or only warnings
  - `503 Service Unavailable`: One or more checks failing

Example response:
```json
{
  "status": "UP",
  "timestamp": "2023-12-07T10:30:00Z",
  "duration": "25ms",
  "checks": {
    "memory": {
      "name": "memory",
      "status": "UP",
      "message": "Memory usage: 45 MB",
      "details": {
        "alloc_mb": 45,
        "sys_mb": 67,
        "gc_runs": 3,
        "goroutines": 12,
        "max_memory_mb": 2048
      },
      "last_checked": "2023-12-07T10:30:00Z",
      "duration": "1ms"
    },
    "database-primary": {
      "name": "database-primary",
      "status": "UP",
      "message": "Database primary is healthy",
      "details": {
        "database": "primary"
      },
      "last_checked": "2023-12-07T10:30:00Z",
      "duration": "2ms"
    }
  },
  "service_info": {
    "service": "go-server",
    "version": "1.2.3",
    "environment": "development"
  }
}
```

### Readiness Check
- **URL**: `GET /health/ready`
- **Purpose**: Indicates if the service is ready to accept traffic
- **Response**: Plain text (`READY` or `NOT READY`)
- **HTTP Status**:
  - `200 OK`: Service is ready
  - `503 Service Unavailable`: Service is not ready

### Liveness Check
- **URL**: `GET /health/live`
- **Purpose**: Indicates if the service is alive (for basic process monitoring)
- **Response**: Plain text (`ALIVE`)
- **HTTP Status**: Always `200 OK` (if the process is running)

## Health Checks Included

### System Health Checks
1. **Memory Usage**: Monitors allocated memory with configurable limits
2. **Goroutine Count**: Tracks goroutine leaks with configurable thresholds
3. **DNS Resolution**: Tests basic network connectivity

### Application Health Checks
1. **Database Connectivity**: Simulates database ping checks
2. **External API Dependencies**: HTTP checks for upstream services
3. **Feature Flags Service**: Custom business logic monitoring
4. **Cache Performance**: Application-specific metrics monitoring

## Configuration Options

### Development vs Production

The example shows how to configure health checks differently for different environments:

```go
// Development configuration
server.Health().ConfigureForDevelopment()

// Production configuration (alternative)
server.Health().ConfigureForProduction()
```

### Adding Custom Checks

```go
// Database check
server.Health().AddDatabaseCheck("primary", func() error {
    return db.Ping()
})

// External service check
server.Health().AddExternalServiceCheck("api-service", "https://api.example.com/health")

// Custom application check
server.Health().AddCustomCheck("custom-logic", func() (health.Status, string, map[string]interface{}) {
    // Your custom check logic here
    return health.StatusUp, "All good", map[string]interface{}{
        "metric": "value",
    }
})
```

## Integration with Monitoring Systems

The health endpoints are designed to work with:

- **Kubernetes**: Use `/health/ready` for readiness probes and `/health/live` for liveness probes
- **Load Balancers**: Use `/health` or `/health/ready` for upstream health checks
- **Monitoring Tools**: Use `/health` for detailed monitoring and alerting
- **Service Discovery**: Use health status for service registration/deregistration

## Best Practices

1. **Keep Checks Fast**: Health checks should complete quickly (< 5 seconds)
2. **Use Appropriate Status Codes**: 
   - `UP`: Everything is working normally
   - `WARNING`: Degraded performance but still functional
   - `DOWN`: Critical failure, service should not receive traffic
3. **Include Useful Details**: Add relevant metrics and diagnostic information
4. **Monitor Dependencies**: Check critical external dependencies
5. **Avoid Cascading Failures**: Don't let health check failures cause more problems

## Customization

You can customize the health checking behavior by:

- Setting custom timeouts: `server.Health().SetTimeout(10 * time.Second)`
- Adding service information: `server.Health().SetServiceInfo("region", "us-west-2")`
- Configuring check-specific limits: `MemoryCheck(1024)` for 1GB limit
- Creating environment-specific configurations

## Example curl Commands

```bash
# Full health check
curl http://localhost:8080/health

# Readiness check
curl http://localhost:8080/health/ready

# Liveness check
curl http://localhost:8080/health/live

# Check specific status codes
curl -w "%{http_code}" -s -o /dev/null http://localhost:8080/health
```