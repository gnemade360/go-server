package goserver

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/yourorg/go-server/filter"
	"github.com/yourorg/go-server/middleware"
	"github.com/yourorg/go-server/router"
	"github.com/yourorg/go-server/static"
)

// Re‑export selected types so existing callers continue to compile.
// New code can import the focused sub‑packages directly.

type (
	Middleware            = middleware.Middleware
	Filter                = filter.Filter
	ConditionalFilter     = filter.ConditionalFilter
	StaticOptions         = router.StaticOptions
	TemplateData          = static.TemplateData
	TemplateStaticHandler = static.TemplateStaticHandler
)

// Option is a function that modifies a Server
type Option func(*Server)

// ServerConfig holds configuration for the Server
type ServerConfig struct {
	// Server address in the format "host:port"
	Addr string

	// Timeouts for the server
	IdleTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Operation timeout for the server
	// If set, the server will automatically shut down after this duration
	// Default is 3 minutes
	Timeout time.Duration

	// CORS configuration
	CORSOrigin string

	// Cache control duration
	CacheDuration time.Duration

	// Enable gzip compression
	EnableGzip bool
}

// Server is a struct that provides server functionality with default middlewares
type Server struct {
	// Configuration for the server
	Config ServerConfig

	// HTTP server
	httpServer *http.Server

	// Router for handling HTTP requests
	router *router.Router

	// Middleware manager
	middlewareManager *middleware.MiddlewareManager

	// Filter manager
	filterManager *filter.FilterManager

	// Timeout for the server's operation
	timeout time.Duration
}

// DefaultOptions returns the default server options
func DefaultOptions() []Option {
	return []Option{
		WithMiddleware(middleware.Gzip()),
		WithMiddleware(middleware.CacheControl(24 * time.Hour)), // Default cache of 24 hours
	}
}

// DefaultServerConfig returns a ServerConfig with default values
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Addr: ":8080",
		//IdleTimeout:   60 * time.Second,
		//ReadTimeout:   15 * time.Second,
		//WriteTimeout:  15 * time.Second,
		CORSOrigin:    "*",
		CacheDuration: 24 * time.Hour,
		EnableGzip:    true,
	}
}

// NewServer creates a new Server instance with default configuration
func NewServer() *Server {
	config := DefaultServerConfig()
	return &Server{
		Config:            config,
		httpServer:        &http.Server{Addr: config.Addr, ReadHeaderTimeout: 15 * time.Second},
		router:            router.NewRouter(),
		middlewareManager: &middleware.MiddlewareManager{},
		filterManager:     &filter.FilterManager{},
		timeout:           config.Timeout,
	}
}

// NewServerWithConfig creates a new Server instance with the provided configuration
func NewServerWithConfig(config ServerConfig) *Server {
	return &Server{
		Config:            config,
		httpServer:        &http.Server{Addr: config.Addr, ReadHeaderTimeout: 15 * time.Second},
		router:            router.NewRouter(),
		middlewareManager: &middleware.MiddlewareManager{},
		filterManager:     &filter.FilterManager{},
		timeout:           config.Timeout,
	}
}

// Router returns the router for this server
func (s *Server) Router() *router.Router {
	return s.router
}

// SetRouter sets the router for this server
func (s *Server) SetRouter(r *router.Router) {
	s.router = r
}

// handlerChain builds the handler chain for the server
func (s *Server) handlerChain() http.Handler {
	h := http.Handler(s.router)
	// Apply filters and middleware to ensure proper connection handling
	h = s.filterManager.ApplyFilters(h)
	h = s.middlewareManager.ApplyMiddleware(h)
	return h
}

// Start starts the server
func (s *Server) Start(ctx context.Context) error {
	s.httpServer.Handler = s.handlerChain()
	errCh := make(chan error, 1)

	go func() { errCh <- s.httpServer.ListenAndServe() }()
	select {
	case <-ctx.Done():
		return s.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// ListenAndServe starts the server and blocks until it's stopped
func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

// Configure applies the given options to the server
func (s *Server) Configure(addr string, opts ...Option) *Server {
	// If addr is provided, use it instead of the one from config
	if addr != "" {
		s.httpServer.Addr = addr
	}

	// Apply configuration options
	if s.Config.Timeout > 0 {
		s.timeout = s.Config.Timeout
	}

	// Add CORS middleware if origin is specified
	if s.Config.CORSOrigin != "" {
		s.middlewareManager.AddMiddleware(middleware.CORS(s.Config.CORSOrigin))
	}

	// Add cache control middleware if duration is specified
	if s.Config.CacheDuration > 0 {
		s.middlewareManager.AddMiddleware(middleware.CacheControl(s.Config.CacheDuration))
	}

	// Add gzip middleware if enabled
	if s.Config.EnableGzip {
		s.middlewareManager.AddMiddleware(middleware.Gzip())
	}

	// Add timeout middleware if timeout is set
	if s.timeout > 0 {
		s.middlewareManager.AddMiddleware(middleware.Timeout(s.timeout))
	}

	// Apply user-provided options
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// WithTimeouts returns an option to set server timeouts
func WithTimeouts(idle, write, read time.Duration) Option {
	return func(s *Server) {
		s.httpServer.IdleTimeout = idle
		s.httpServer.WriteTimeout = write
		s.httpServer.ReadTimeout = read
	}
}

// WithMiddleware returns an option to add middleware to the server
func WithMiddleware(mw ...middleware.Middleware) Option {
	return func(s *Server) {
		s.middlewareManager.AddMiddleware(mw...)
	}
}

// WithFilter returns an option to add a filter to the server
func WithFilter(f ...filter.Filter) Option {
	return func(s *Server) {
		s.filterManager.AddFilter(f...)
	}
}

// WithFilterFn returns an option to add a filter function to the server
func WithFilterFn(order int, fn filter.FilterFn) Option {
	return func(s *Server) {
		s.filterManager.AddFilterFn(order, fn)
	}
}

// WithTimeout returns an option to set the server's operation timeout
func WithTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.timeout = timeout
		// Add timeout middleware if timeout is set
		if timeout > 0 {
			s.middlewareManager.AddMiddleware(middleware.Timeout(timeout))
		}
	}
}

// NewFilterFn creates a new filter function
func (s *Server) NewFilterFn(order int, fn filter.FilterFn) filter.Filter {
	return filter.NewFilterFn(order, fn)
}

// NewTemplateStaticHandler creates a new template static handler
func (s *Server) NewTemplateStaticHandler(dir, templateFile string, data static.TemplateData) *static.TemplateStaticHandler {
	return static.NewTemplateStaticHandler(dir, templateFile, data)
}

// SecureJoinPath safely joins a base directory with a relative path and returns an absolute path.
// It prevents directory traversal attacks by ensuring the resulting path is within the base directory.
// Returns an error if the path would escape the base directory.
func (s *Server) SecureJoinPath(baseDir, relativePath string) (string, error) {
	// Clean both paths to normalize them
	baseDir = filepath.Clean(baseDir)
	relativePath = filepath.Clean(relativePath)

	// Remove any leading slash from the relative path to ensure it's truly relative
	relativePath = strings.TrimPrefix(relativePath, "/")

	// Join the paths
	joinedPath := filepath.Join(baseDir, relativePath)

	// Convert to absolute path
	absPath, err := filepath.Abs(joinedPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Convert base directory to absolute path for comparison
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute base directory: %w", err)
	}

	// Ensure the resulting path is within the base directory
	if !strings.HasPrefix(absPath, absBaseDir) {
		return "", fmt.Errorf("path traversal detected: %s is not within %s", absPath, absBaseDir)
	}

	return absPath, nil
}

// For backward compatibility, create a global instance
var defaultServer = NewServer()

// New creates a new server with default middlewares (Gzip, CacheControl)
func New(addr string, opts ...Option) *Server {
	return defaultServer.Configure(addr, opts...)
}

// NewWithConfig creates a new server with the provided configuration
func NewWithConfig(config ServerConfig, addr string, opts ...Option) *Server {
	server := NewServerWithConfig(config)
	return server.Configure(addr, opts...)
}

// Option functions are defined above

// GetDefaultConfig returns the default server configuration
func GetDefaultConfig() ServerConfig {
	return DefaultServerConfig()
}

// ---------------------------------------------------------------------
// Sub‑package exports for ease of use (optional):
// ---------------------------------------------------------------------
var (
	NewFilterFn              = filter.NewFilterFn
	Gzip                     = middleware.Gzip
	CacheControl             = middleware.CacheControl
	CORS                     = middleware.CORS
	NewTemplateStaticHandler = static.NewTemplateStaticHandler
)

// SecureJoinPath safely joins a base directory with a relative path and returns an absolute path.
// It prevents directory traversal attacks by ensuring the resulting path is within the base directory.
// Returns an error if the path would escape the base directory.
func SecureJoinPath(baseDir, relativePath string) (string, error) {
	return defaultServer.SecureJoinPath(baseDir, relativePath)
}
