package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"kuberoot/internal/api"
	"kuberoot/internal/auth"
	"kuberoot/internal/k8s"
	"kuberoot/internal/store"
)

func main() {
	cs, err := k8s.NewClientset()
	if err != nil {
		log.Fatalf("failed to create clientset: %v", err)
	}

	clusterID := envOrDefault("KUBEROOT_CLUSTER_ID", "local")

	diagnosisStore := store.NewNoopStore()
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		postgresStore, pgErr := store.NewPostgresStore(ctx, databaseURL)
		if pgErr != nil {
			log.Fatalf("failed to initialize postgres store: %v", pgErr)
		}
		diagnosisStore = postgresStore
		log.Printf("postgres persistence enabled")
		log.Printf("API key authentication enabled")
	} else {
		log.Printf("postgres persistence disabled (DATABASE_URL not set)")
		log.Printf("API key authentication disabled (local mode)")
	}

	handler := api.NewHandler(cs, diagnosisStore, clusterID)

	var finalHandler http.Handler = http.NewServeMux()
	mux := finalHandler.(*http.ServeMux)
	mux.HandleFunc("/health", handler.Health)
	mux.HandleFunc("/diagnose", handler.Diagnose)
	mux.HandleFunc("/diagnose/history", handler.DiagnoseHistory)
	mux.HandleFunc("/api/v1/agent/report", handler.AgentReport)

	if _, hasDB := diagnosisStore.(*store.PostgresStore); hasDB {
		authMiddleware := auth.APIKeyMiddleware(diagnosisStore)
		finalHandler = authMiddleware(finalHandler)
	} else {
		localAuthMiddleware := auth.LocalModeMiddleware()
		finalHandler = localAuthMiddleware(finalHandler)
	}

	server := &http.Server{
		Addr:    ":8080",
		Handler: finalHandler,
	}

	log.Printf("server listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
