package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"
)

type GenerateKeyRequest struct {
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name,omitempty"`
	ClusterID      string `json:"clusterId,omitempty"`
}

type GenerateKeyResponse struct {
	APIKey         string `json:"apiKey"`
	OrganizationID string `json:"organizationId"`
	ClusterID      string `json:"clusterId,omitempty"`
	Active         bool   `json:"active"`
	CreatedAt      string `json:"createdAt"`
}

func (h *Handler) GenerateAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	internalToken := strings.TrimSpace(os.Getenv("INTERNAL_API_TOKEN"))
	if internalToken == "" {
		http.Error(w, "internal key generation not configured", http.StatusServiceUnavailable)
		return
	}

	providedToken := strings.TrimSpace(r.Header.Get("X-Internal-Token"))
	if providedToken == "" || providedToken != internalToken {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var payload GenerateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	payload.OrganizationID = strings.TrimSpace(payload.OrganizationID)
	payload.Name = strings.TrimSpace(payload.Name)
	payload.ClusterID = strings.TrimSpace(payload.ClusterID)
	if payload.OrganizationID == "" {
		http.Error(w, "organizationId required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	rawKey, err := h.store.CreateAPIKey(ctx, payload.OrganizationID, payload.Name)
	if err != nil {
		http.Error(w, "failed to create API key: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if payload.ClusterID != "" {
		if err := h.store.RegisterCluster(ctx, payload.OrganizationID, payload.ClusterID); err != nil {
			http.Error(w, "failed to register cluster: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	response := GenerateKeyResponse{
		APIKey:         rawKey,
		OrganizationID: payload.OrganizationID,
		ClusterID:      payload.ClusterID,
		Active:         true,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}
