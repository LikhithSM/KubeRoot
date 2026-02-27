package analyzer

import (
	"strings"
	"time"

	"kuberoot/internal/k8s"
)

type Diagnosis struct {
	OrganizationID string    `json:"organizationId"`
	ClusterID      string    `json:"clusterId"`
	PodName        string    `json:"podName"`
	Namespace      string    `json:"namespace"`
	FailureType    string    `json:"failureType"`
	LikelyCause    string    `json:"likelyCause"`
	SuggestedFix   string    `json:"suggestedFix"`
	Confidence     string    `json:"confidence"`
	Events         []string  `json:"events"`
	Timestamp      time.Time `json:"timestamp"`
}

type Rule struct {
	FailureType  string
	LikelyCause  string
	SuggestedFix string
	Confidence   string
}

var v1Rules = []Rule{
	{
		FailureType:  "CrashLoopBackOff",
		LikelyCause:  "Application crash or configuration error",
		SuggestedFix: "Check application logs; verify environment variables; validate ConfigMap/Secret mounts",
		Confidence:   "medium",
	},
	{
		FailureType:  "OOMKilled",
		LikelyCause:  "Container exceeded memory limit",
		SuggestedFix: "Increase memory limit; inspect memory usage; investigate memory leaks",
		Confidence:   "high",
	},
	{
		FailureType:  "ImagePullBackOff",
		LikelyCause:  "Image pull failed due to image name, registry access, or credentials",
		SuggestedFix: "Verify image name; check registry access; validate pull secret for private registry",
		Confidence:   "high",
	},
}

func DiagnoseFailures(orgID, clusterID string, failures []k8s.PodFailure) []Diagnosis {
	ruleMap := make(map[string]Rule, len(v1Rules))
	for _, rule := range v1Rules {
		ruleMap[rule.FailureType] = rule
	}

	out := make([]Diagnosis, 0, len(failures))
	for _, failure := range failures {
		for _, failureType := range failure.Types {
			rule, exists := ruleMap[failureType]
			if !exists {
				continue
			}

			out = append(out, Diagnosis{
				OrganizationID: orgID,
				ClusterID:      clusterID,
				PodName:        failure.Name,
				Namespace:      failure.Namespace,
				FailureType:    rule.FailureType,
				LikelyCause:    rule.LikelyCause,
				SuggestedFix:   rule.SuggestedFix,
				Confidence:     enrichConfidence(rule.Confidence, rule.FailureType, failure.Events),
				Events:         failure.Events,
				Timestamp:      time.Now().UTC(),
			})
		}
	}

	return out
}

func enrichConfidence(base, failureType string, events []string) string {
	baseScore := confidenceScore(base)
	evidenceScore := 0

	for _, event := range events {
		lowerEvent := strings.ToLower(event)
		switch failureType {
		case "CrashLoopBackOff":
			if strings.Contains(lowerEvent, "back-off restarting") || strings.Contains(lowerEvent, "crashloopbackoff") || strings.Contains(lowerEvent, "failed") {
				evidenceScore = maxInt(evidenceScore, 1)
			}
		case "OOMKilled":
			if strings.Contains(lowerEvent, "oomkilled") || strings.Contains(lowerEvent, "out of memory") || strings.Contains(lowerEvent, "killing") {
				evidenceScore = maxInt(evidenceScore, 1)
			}
		case "ImagePullBackOff":
			if strings.Contains(lowerEvent, "failed to pull image") || strings.Contains(lowerEvent, "pull access denied") || strings.Contains(lowerEvent, "manifest unknown") || strings.Contains(lowerEvent, "imagepullbackoff") {
				evidenceScore = maxInt(evidenceScore, 1)
			}
		}
	}

	if len(events) == 0 {
		baseScore = maxInt(1, baseScore-1)
	}

	finalScore := baseScore + evidenceScore
	if finalScore > 3 {
		finalScore = 3
	}
	return scoreConfidence(finalScore)
}

func confidenceScore(confidence string) int {
	switch strings.ToLower(confidence) {
	case "high":
		return 3
	case "medium":
		return 2
	default:
		return 1
	}
}

func scoreConfidence(score int) string {
	if score >= 3 {
		return "high"
	}
	if score == 2 {
		return "medium"
	}
	return "low"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
