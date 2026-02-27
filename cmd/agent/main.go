package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"kuberoot/internal/k8s"
)

type AgentPayload struct {
	ClusterID string       `json:"clusterId"`
	Timestamp time.Time    `json:"timestamp"`
	Failures  []k8s.PodFailure `json:"failures"`
}

type AgentConfig struct {
	BackendURL  string
	APIKey      string
	ClusterID   string
	PollInterval time.Duration
}

func main() {
	// Parse flags
	backendURL := flag.String("backend", os.Getenv("KUBEROOT_BACKEND_URL"), "Backend URL (env: KUBEROOT_BACKEND_URL)")
	apiKey := flag.String("api-key", os.Getenv("KUBEROOT_API_KEY"), "API Key (env: KUBEROOT_API_KEY)")
	clusterID := flag.String("cluster-id", os.Getenv("KUBEROOT_CLUSTER_ID"), "Cluster ID")
	pollInterval := flag.Duration("poll-interval", 30*time.Second, "Poll interval for failures")
	flag.Parse()

	// Validate config
	if *backendURL == "" {
		log.Fatal("--backend or KUBEROOT_BACKEND_URL required")
	}
	if *apiKey == "" {
		log.Fatal("--api-key or KUBEROOT_API_KEY required")
	}
	if *clusterID == "" {
		*clusterID = "local"
	}

	config := AgentConfig{
		BackendURL:   *backendURL,
		APIKey:       *apiKey,
		ClusterID:    *clusterID,
		PollInterval: *pollInterval,
	}

	log.Printf("Kuberoot Agent Starting")
	log.Printf("  Backend: %s", config.BackendURL)
	log.Printf("  Cluster: %s", config.ClusterID)
	log.Printf("  Poll Interval: %v", config.PollInterval)

	// Try in-cluster config first
	var cs *kubernetes.Clientset
	var err error

	kubeConfig, inClusterErr := rest.InClusterConfig()
	if inClusterErr == nil {
		cs, err = kubernetes.NewForConfig(kubeConfig)
		if err != nil {
			log.Fatalf("Failed to create clientset from in-cluster config: %v", err)
		}
		log.Printf("✓ Using in-cluster Kubernetes config")
	} else {
		log.Printf("ℹ Not running in-cluster, using kubeconfig...")
		cs, err = k8s.NewClientset()
		if err != nil {
			log.Fatalf("Failed to create clientset: %v", err)
		}
	}

	log.Printf("✓ Connected to Kubernetes cluster")

	// Start detection loop
	runAgentLoop(cs, config)
}

func runAgentLoop(cs *kubernetes.Clientset, config AgentConfig) {
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()

	// Run once immediately
	detectAndReport(cs, config)

	// Then run on interval
	for range ticker.C {
		detectAndReport(cs, config)
	}
}

func detectAndReport(cs *kubernetes.Clientset, config AgentConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Detect failures
	failures, err := k8s.GetFailedPods(ctx, cs)
	if err != nil {
		log.Printf("❌ Failed to detect failures: %v", err)
		return
	}

	log.Printf("📊 Detected %d failures", len(failures))

	// Build payload
	payload := AgentPayload{
		ClusterID: config.ClusterID,
		Timestamp: time.Now().UTC(),
		Failures:  failures,
	}

	// Send to backend
	if err := sendReport(config, payload); err != nil {
		log.Printf("❌ Failed to send report: %v", err)
		return
	}

	log.Printf("✓ Report sent to backend (%d failures)", len(failures))
}

func sendReport(config AgentConfig, payload AgentPayload) error {
	// Marshal payload
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	// Create request
	url := fmt.Sprintf("%s/api/v1/agent/report", config.BackendURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", config.APIKey)

	// Send request
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("backend returned %d", resp.StatusCode)
	}

	return nil
}
