package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kuberoot/internal/analyzer"
	"kuberoot/internal/auth"
	"kuberoot/internal/store"
)

type Handler struct {
	store     store.DiagnosisStore
	clusterID string
}

type DiagnoseHistoryResponse struct {
	Cluster string               `json:"cluster"`
	Count   int                  `json:"count"`
	Items   []analyzer.Diagnosis `json:"items"`
}

func NewHandler(diagnosisStore store.DiagnosisStore, clusterID string) *Handler {
	return &Handler{
		store:     diagnosisStore,
		clusterID: clusterID,
	}
}

func (h *Handler) DiagnoseHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID := auth.GetOrganizationID(r.Context())
	if orgID == "" {
		http.Error(w, "missing organization context", http.StatusInternalServerError)
		return
	}

	filter := store.DiagnosisHistoryFilter{Limit: 50}
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil || parsedLimit <= 0 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		if parsedLimit > 200 {
			parsedLimit = 200
		}
		filter.Limit = parsedLimit
	}

	if failureType := strings.TrimSpace(r.URL.Query().Get("failureType")); failureType != "" {
		filter.FailureType = failureType
	}

	if namespace := strings.TrimSpace(r.URL.Query().Get("namespace")); namespace != "" {
		filter.Namespace = namespace
	}

	if rawSince := strings.TrimSpace(r.URL.Query().Get("since")); rawSince != "" {
		since, err := time.Parse(time.RFC3339, rawSince)
		if err != nil {
			http.Error(w, "invalid since (use RFC3339)", http.StatusBadRequest)
			return
		}
		filter.Since = &since
	}

	if rawUntil := strings.TrimSpace(r.URL.Query().Get("until")); rawUntil != "" {
		until, err := time.Parse(time.RFC3339, rawUntil)
		if err != nil {
			http.Error(w, "invalid until (use RFC3339)", http.StatusBadRequest)
			return
		}
		filter.Until = &until
	}

	if filter.Since != nil && filter.Until != nil && filter.Since.After(*filter.Until) {
		http.Error(w, "since must be earlier than or equal to until", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	history, err := h.store.ListDiagnoses(ctx, orgID, h.clusterID, filter)
	if err != nil {
		http.Error(w, "failed to load diagnosis history: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := DiagnoseHistoryResponse{
		Cluster: h.clusterID,
		Count:   len(history),
		Items:   history,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Ready   bool   `json:"ready"`
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Health endpoint requires no authentication
	response := HealthResponse{
		Status:  "ok",
		Version: "1.0.0",
		Ready:   true,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
