package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"kuberoot/internal/analyzer"
	"kuberoot/internal/auth"
	"kuberoot/internal/k8s" // Still needed for PodFailure type
)

// AgentPayload is what the agent sends (struct here is fine, payload format matters)
type AgentPayload struct {
	ClusterID string           `json:"clusterId"`
	Timestamp time.Time        `json:"timestamp"`
	Failures  []k8s.PodFailure `json:"failures"`
}

// AgentReportResponse is what we return
type AgentReportResponse struct {
	Status  string `json:"status"`
	ID      string `json:"id"`
	Message string `json:"message,omitempty"`
}

// AgentReport receives failure reports from cluster agents
// Note: SaveDiagnoses internally calls RegisterCluster to track cluster health
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

	// Run analyzer
	diagnoses := analyzer.DiagnoseFailures(orgID, payload.ClusterID, payload.Failures)
	log.Printf("[AGENT] org=%s cluster=%s failures=%d diagnoses=%d", orgID, payload.ClusterID, len(payload.Failures), len(diagnoses))

	newIssues := make([]analyzer.Diagnosis, 0, len(diagnoses))
	for _, d := range diagnoses {
		seenRecently, seenErr := h.store.FailureSeenRecently(ctx, orgID, payload.ClusterID, d.Namespace, d.PodName, d.FailureType, 10*time.Minute)
		if seenErr != nil {
			log.Printf("[WARN] recent failure check error for %s/%s %s: %v", d.Namespace, d.PodName, d.FailureType, seenErr)
			continue
		}
		if !seenRecently {
			newIssues = append(newIssues, d)
		}
	}

	if len(diagnoses) > 0 {
		for _, d := range diagnoses {
			log.Printf("[DIAGNOSIS] %s/%s: %s (confidence=%s)", d.Namespace, d.PodName, d.FailureType, d.Confidence)
		}
	}

	// Store diagnoses (this also registers/updates cluster in DB)
	if err := h.store.SaveDiagnoses(ctx, orgID, payload.ClusterID, diagnoses); err != nil {
		log.Printf("[ERROR] failed to store diagnoses: %v", err)
		http.Error(w, "failed to store diagnoses: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if webhook := os.Getenv("SLACK_WEBHOOK_URL"); webhook != "" && len(newIssues) > 0 {
		if err := notifySlack(webhook, payload.ClusterID, newIssues); err != nil {
			log.Printf("[WARN] slack notification failed: %v", err)
		}
	}

	// Return success
	response := AgentReportResponse{
		Status:  "accepted",
		ID:      payload.ClusterID,
		Message: "processed diagnoses",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

func notifySlack(webhookURL, clusterID string, diagnoses []analyzer.Diagnosis) error {
	maxItems := 5
	if len(diagnoses) < maxItems {
		maxItems = len(diagnoses)
	}

	lines := make([]string, 0, maxItems+2)
	lines = append(lines, "Kuberoot detected new active failures")
	lines = append(lines, "Cluster: "+clusterID)
	for i := 0; i < maxItems; i++ {
		d := diagnoses[i]
		lines = append(lines, "- "+d.Namespace+"/"+d.PodName+" | "+d.FailureType+" | "+strings.ToUpper(d.Confidence))
	}
	if len(diagnoses) > maxItems {
		lines = append(lines, "...and "+itoa(len(diagnoses)-maxItems)+" more")
	}

	body, err := json.Marshal(map[string]string{
		"text": strings.Join(lines, "\n"),
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack webhook status %d", resp.StatusCode)
	}

	return nil
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for v > 0 {
		d := v % 10
		buf = append([]byte{byte('0' + d)}, buf...)
		v /= 10
	}
	return string(buf)
}
