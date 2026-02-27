package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"kuberoot/internal/analyzer"
	"kuberoot/internal/auth"
	"kuberoot/internal/k8s"
)

// AgentPayload is what the agent sends
type AgentPayload struct {
	ClusterID string              `json:"clusterId"`
	Timestamp time.Time           `json:"timestamp"`
	Failures  []k8s.PodFailure    `json:"failures"`
}

// AgentReportResponse is what we return
type AgentReportResponse struct {
	Status  string `json:"status"`
	ID      string `json:"id"`
	Message string `json:"message,omitempty"`
}

// AgentReport receives failure reports from cluster agents
func (h *Handler) AgentReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get orgID from auth middleware (already validated X-API-Key)
	orgID := auth.GetOrganizationID(r.Context())
	if orgID == "" {
		http.Error(w, "missing organization context", http.StatusUnauthorized)
		return
	}

	// Parse payload
	var payload AgentPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate cluster ID
	if payload.ClusterID == "" {
		http.Error(w, "clusterId required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Run analyzer (same logic as /diagnose endpoint, but with agent-provided failures)
	diagnoses := analyzer.DiagnoseFailures(orgID, payload.ClusterID, payload.Failures)
	log.Printf("[AGENT] received report: org=%s cluster=%s failures=%d diagnoses=%d", orgID, payload.ClusterID, len(payload.Failures), len(diagnoses))
	if len(diagnoses) > 0 {
		for _, d := range diagnoses {
			log.Printf("[AGENT-DIAGNOSIS]   %s/%s: %s (confidence=%s)", d.Namespace, d.PodName, d.FailureType, d.Confidence)
		}
	}

	// Store diagnoses
	if err := h.store.SaveDiagnoses(ctx, orgID, payload.ClusterID, diagnoses); err != nil {
		http.Error(w, "failed to store diagnoses: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success
	response := AgentReportResponse{
		Status:  "accepted",
		ID:      payload.ClusterID,
		Message: "processed " + string(rune(len(diagnoses))) + " diagnoses",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
