// handlers/api/server.go
package api

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/nijaru/yt-text/config"
	"github.com/nijaru/yt-text/middleware"
	"github.com/nijaru/yt-text/services/summary"
	"github.com/nijaru/yt-text/services/video"
	"github.com/nijaru/yt-text/validation"
	"github.com/sirupsen/logrus"
)

type Server struct {
	video     *VideoHandler
	summary   *SummaryHandler
	config    *config.Config
	logger    *logrus.Logger
	server    *http.Server
	startTime time.Time
}

type ServerOption func(*Server)

// NewServer creates a new API server with the provided services and options
func NewServer(cfg *config.Config, opts ...ServerOption) *Server {
	s := &Server{
		config:    cfg,
		logger:    logrus.StandardLogger(),
		startTime: time.Now(),
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// Create HTTP server
	s.server = &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      s.routes(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	return s
}

// WithServices sets up the handlers with the provided services
func WithServices(videoSvc video.Service, summarySvc summary.Service) ServerOption {
	return func(s *Server) {
		validator := validation.NewValidator(s.config)
		s.video = NewVideoHandler(videoSvc, validator)
		s.summary = NewSummaryHandler(summarySvc, validator)
	}
}

// WithLogger sets a custom logger for the server
func WithLogger(logger *logrus.Logger) ServerOption {
	return func(s *Server) {
		s.logger = logger
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.WithField("port", s.config.ServerPort).Info("Starting server")
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down server...")
	return s.server.Shutdown(ctx)
}

// routes sets up all the routes for the API
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// API v1 routes
	s.addV1Routes(mux)

	// Health check
	mux.HandleFunc("GET /health", s.handleHealth)

	// Apply middleware stack
	return s.middleware(mux)
}

// addV1Routes adds all the v1 API routes
func (s *Server) addV1Routes(mux *http.ServeMux) {
	const v1Prefix = "/api/v1"

	// Video transcription endpoints
	mux.HandleFunc("POST "+v1Prefix+"/transcribe", s.video.HandleCreateTranscription)
	mux.HandleFunc("GET "+v1Prefix+"/transcription", s.video.HandleGetTranscription)
	mux.HandleFunc("POST "+v1Prefix+"/transcribe/cancel", s.video.HandleCancelTranscription)
	mux.HandleFunc("GET "+v1Prefix+"/transcribe/status/{id}", s.video.HandleGetStatus)

	// Summary endpoints
	mux.HandleFunc("POST "+v1Prefix+"/summary", s.summary.HandleCreateSummary)
	mux.HandleFunc("GET "+v1Prefix+"/summary", s.summary.HandleGetSummary)
	mux.HandleFunc("GET "+v1Prefix+"/summary/status/{id}", s.summary.HandleGetSummaryStatus)
}

// middleware sets up the middleware chain
func (s *Server) middleware(handler http.Handler) http.Handler {
	var rateLimiter middleware.RateLimiter
	if s.config.RateLimit.Enabled {
		rateLimiter = middleware.NewRateLimiter(
			s.config.RateLimit.RequestsPerMinute,
			s.config.RateLimit.BurstSize,
		)
	}

	middlewares := []func(http.Handler) http.Handler{
		middleware.Recovery(s.logger),
		middleware.RequestID(),
		middleware.Logging(s.logger),
		middleware.CORS(s.config.CORS),
		middleware.Timeout(s.config.RequestTimeout),
	}

	if rateLimiter != nil {
		middlewares = append(middlewares, rateLimiter.Middleware)
	}

	return middleware.Chain(handler, middlewares...)
}

// handleHealth handles the health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC(),
		"version":   s.config.Version,
		"uptime":    time.Since(s.startTime).String(),
	}

	// Add additional health metrics if debug is enabled
	if s.config.Debug {
		status["debug"] = true
		status["goroutines"] = runtime.NumGoroutine()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		status["memory"] = map[string]interface{}{
			"allocated": m.Alloc,
			"total":     m.TotalAlloc,
			"system":    m.Sys,
			"gc_cycles": m.NumGC,
		}
	}

	respondJSON(w, r, http.StatusOK, status)
}

// handleError is a helper function for consistent error responses
func (s *Server) handleError(w http.ResponseWriter, r *http.Request, err error) {
	// Log error with request context
	s.logger.WithFields(logrus.Fields{
		"request_id": r.Context().Value("request_id"),
		"method":     r.Method,
		"path":       r.URL.Path,
		"error":      err,
	}).Error("Request error")

	respondError(w, r, err)
}
