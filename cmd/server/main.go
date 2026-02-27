package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"kuberoot/internal/api"
	"kuberoot/internal/auth"
	"kuberoot/internal/store"
)

func main() {
	// REQUIRE DATABASE_URL - No local mode fallback
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatalf("FATAL: DATABASE_URL environment variable is required in SaaS mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	postgresStore, pgErr := store.NewPostgresStore(ctx, databaseURL)
	if pgErr != nil {
		log.Fatalf("FATAL: failed to initialize postgres store: %v", pgErr)
	}

	clusterID := os.Getenv("KUBEROOT_CLUSTER_ID")
	if clusterID == "" {
		clusterID = "saas-backend"
	}

	// Create handler WITHOUT k8s clientset (SaaS mode - no cluster access)
	handler := api.NewHandler(postgresStore, clusterID)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handler.Health)
	mux.HandleFunc("/diagnose/history", handler.DiagnoseHistory)
	mux.HandleFunc("/api/v1/agent/report", handler.AgentReport)
	// NOTE: /diagnose removed - not available in SaaS mode (only agent-pushed data)

	// Apply middleware stack in reverse order (innermost first)
	var httpHandler http.Handler = mux

	// 1. Panic recovery (must be outermost - catches panics from all layers)
	httpHandler = panicRecoveryMiddleware()(httpHandler)

	// 2. Structured logging
	httpHandler = loggingMiddleware()(httpHandler)

	// 3. CORS (allow cross-origin requests)
	httpHandler = corsMiddleware()(httpHandler)

	// 4. Body size limit (1MB max)
	httpHandler = bodySizeLimitMiddleware(1024 * 1024)(httpHandler)

	// 5. Request timeout (10 seconds max)
	httpHandler = timeoutMiddleware(10 * time.Second)(httpHandler)

	// 6. API Key validation (required for all endpoints except /health)
	httpHandler = auth.APIKeyMiddleware(postgresStore)(httpHandler)

	// PORT from environment (Railway/Heroku sets this)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	server := &http.Server{
		Addr:         addr,
		Handler:      httpHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Printf("🚀 Kuberoot backend starting on %s (SaaS mode, database-backed)", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

// panicRecoveryMiddleware recovers from panics and returns 500
// Ensures one bad request doesn't crash the entire process
func panicRecoveryMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("PANIC RECOVERED: %v | Path: %s | Method: %s", rec, r.RequestURI, r.Method)
					w.Header().Set("Content-Type", "application/json")
					http.Error(w, `{"status":"error","message":"internal server error"}`, http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// timeoutMiddleware cancels request after duration
func timeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// bodySizeLimitMiddleware rejects requests larger than limit
func bodySizeLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// corsMiddleware sets CORS headers
// In production, use CORS_ORIGIN env var to restrict to your frontend domain
func corsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := os.Getenv("CORS_ORIGIN")
			if origin == "" {
				origin = "*" // Allow all in local dev, restrict in production
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// loggingMiddleware logs all requests with timing
func loggingMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			log.Printf("[%s] %s | %v", r.Method, r.RequestURI, time.Since(start))
		})
	}
}
