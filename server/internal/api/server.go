package api

import (
	"net/http"
	"time"

	"github.com/PlainFunction/mistokenly/internal/common/config"
	"github.com/gorilla/mux"
)

type Server struct {
	config  *config.Config
	router  *mux.Router
	handler *Handler
}

func NewServer(cfg *config.Config) *Server {
	server := &Server{
		config: cfg,
		router: mux.NewRouter(),
	}

	// Initialize handler with dependencies
	server.handler = NewHandler(cfg)

	// Setup routes
	server.setupRoutes()

	return server
}

func (s *Server) setupRoutes() {
	// Health check
	s.router.HandleFunc("/health", s.handler.HealthCheck).Methods("GET")

	// API v1 routes
	api := s.router.PathPrefix("/v1").Subrouter()

	// PII operations
	api.HandleFunc("/tokenize", s.handler.Tokenize).Methods("POST")
	api.HandleFunc("/detokenize", s.handler.Detokenize).Methods("POST")

	// Metrics endpoint (Prometheus)
	api.HandleFunc("/metrics", s.handler.Metrics).Methods("GET")

	// Audit logs endpoint
	api.HandleFunc("/audit/logs", s.handler.GetAuditLogs).Methods("GET")

	// Middleware
	s.router.Use(loggingMiddleware)
	s.router.Use(corsMiddleware)
}

func (s *Server) Start() error {
	server := &http.Server{
		Addr:         ":" + s.config.APIPort,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return server.ListenAndServe()
}

// Middleware functions
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)

		// Simple logging - replace with structured logging in production
		println("[API]", r.Method, r.URL.Path, "-", duration.String())
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
