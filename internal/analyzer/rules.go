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
	Container      string    `json:"container"`
	Image          string    `json:"image"`
	RestartCount   int32     `json:"restartCount"`
	FailureType    string    `json:"failureType"`
	LikelyCause    string    `json:"likelyCause"`
	SuggestedFix   string    `json:"suggestedFix"`
	Confidence     string    `json:"confidence"`
	ConfidenceNote string    `json:"confidenceNote"`
	Evidence       []string  `json:"evidence"`
	QuickCommands  []string  `json:"quickCommands"`
	Context        []string  `json:"context"`
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
		LikelyCause:  "Application exits soon after startup",
		SuggestedFix: "Inspect previous container logs and validate startup configuration",
		Confidence:   "medium",
	},
	{
		FailureType:  "OOMKilled",
		LikelyCause:  "Container exceeded memory limit",
		SuggestedFix: "Increase memory limit and review application memory usage",
		Confidence:   "high",
	},
	{
		FailureType:  "ImagePullBackOff",
		LikelyCause:  "Image cannot be pulled from registry",
		SuggestedFix: "Verify image name; check registry access; validate pull secret for private registry",
		Confidence:   "high",
	},
	{
		FailureType:  "FailedScheduling",
		LikelyCause:  "Pod could not be scheduled due to cluster constraints",
		SuggestedFix: "Inspect node capacity, taints, and pod resource requests",
		Confidence:   "high",
	},
	{
		FailureType:  "ReadinessProbeFailed",
		LikelyCause:  "Readiness probe checks are failing",
		SuggestedFix: "Validate readiness endpoint and tune probe configuration",
		Confidence:   "high",
	},
	{
		FailureType:  "LivenessProbeFailed",
		LikelyCause:  "Liveness probe checks are failing",
		SuggestedFix: "Validate liveness endpoint and tune probe configuration",
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

			evidence := buildEvidence(failureType, failure)
			context := buildContextSignals(failure)
			likelyCause := deriveLikelyCause(rule.LikelyCause, failureType, failure, evidence)
			suggestedFix := deriveSuggestedFix(rule.SuggestedFix, failureType, failure)
			quickCommands := buildQuickCommands(failureType, failure)
			confidence, confidenceNote := enrichConfidence(rule.Confidence, failureType, failure, evidence)

			out = append(out, Diagnosis{
				OrganizationID: orgID,
				ClusterID:      clusterID,
				PodName:        failure.Name,
				Namespace:      failure.Namespace,
				Container:      failure.Container,
				Image:          failure.Image,
				RestartCount:   failure.RestartCount,
				FailureType:    rule.FailureType,
				LikelyCause:    likelyCause,
				SuggestedFix:   suggestedFix,
				Confidence:     confidence,
				ConfidenceNote: confidenceNote,
				Evidence:       evidence,
				QuickCommands:  quickCommands,
				Context:        context,
				Events:         failure.Events,
				Timestamp:      time.Now().UTC(),
			})
		}
	}

	return out
}

func enrichConfidence(base, failureType string, failure k8s.PodFailure, evidence []string) (string, string) {
	baseScore := confidenceScore(base)
	evidenceScore := 0
	reasons := make([]string, 0, 3)

	for _, event := range failure.Events {
		lowerEvent := strings.ToLower(event)
		switch failureType {
		case "CrashLoopBackOff":
			if strings.Contains(lowerEvent, "back-off restarting") || strings.Contains(lowerEvent, "crashloopbackoff") {
				evidenceScore = maxInt(evidenceScore, 1)
				reasons = append(reasons, "kubelet reported repeated restart backoff")
			}
		case "OOMKilled":
			if strings.Contains(lowerEvent, "oomkilled") || strings.Contains(lowerEvent, "out of memory") || strings.Contains(lowerEvent, "killing") {
				evidenceScore = maxInt(evidenceScore, 1)
				reasons = append(reasons, "events include out-of-memory signal")
			}
		case "ImagePullBackOff":
			if strings.Contains(lowerEvent, "failed to pull image") || strings.Contains(lowerEvent, "pull access denied") || strings.Contains(lowerEvent, "manifest unknown") || strings.Contains(lowerEvent, "imagepullbackoff") {
				evidenceScore = maxInt(evidenceScore, 1)
				reasons = append(reasons, "registry pull failure appears in pod events")
			}
		case "FailedScheduling":
			if strings.Contains(lowerEvent, "insufficient") || strings.Contains(lowerEvent, "didn't match") || strings.Contains(lowerEvent, "taint") {
				evidenceScore = maxInt(evidenceScore, 1)
				reasons = append(reasons, "scheduler event reports node/resource constraints")
			}
		case "ReadinessProbeFailed", "LivenessProbeFailed":
			if strings.Contains(lowerEvent, "probe failed") || strings.Contains(lowerEvent, "unhealthy") {
				evidenceScore = maxInt(evidenceScore, 1)
				reasons = append(reasons, "kubelet reported probe failure")
			}
		}
	}

	if len(failure.Events) == 0 {
		baseScore = maxInt(1, baseScore-1)
		reasons = append(reasons, "no recent events were captured")
	}

	if len(evidence) >= 3 {
		evidenceScore = maxInt(evidenceScore, 1)
		reasons = append(reasons, "multiple technical signals agree")
	}

	if failure.RestartCount >= 5 && failureType == "CrashLoopBackOff" {
		evidenceScore = maxInt(evidenceScore, 1)
		reasons = append(reasons, "high restart count indicates persistent crash")
	}

	if failure.ExitCode > 0 && failureType == "CrashLoopBackOff" {
		reasons = append(reasons, "non-zero process exit code observed")
	}

	finalScore := baseScore + evidenceScore
	if finalScore > 3 {
		finalScore = 3
	}

	note := "limited direct signals"
	if len(reasons) > 0 {
		note = strings.Join(uniqueStrings(reasons), "; ")
	}

	return scoreConfidence(finalScore), note
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

func buildEvidence(failureType string, failure k8s.PodFailure) []string {
	evidence := make([]string, 0, 8)

	if failure.Image != "" {
		evidence = append(evidence, "Image: "+failure.Image)
	}
	if failure.Container != "" {
		evidence = append(evidence, "Container: "+failure.Container)
	}
	if failure.RestartCount > 0 {
		evidence = append(evidence, "Restart count: "+itoa32(failure.RestartCount))
	}
	if failure.ContainerState != "" && failure.ContainerState != "Unknown" {
		evidence = append(evidence, "Container state: "+failure.ContainerState)
	}
	if failure.WaitingReason != "" {
		evidence = append(evidence, "Waiting reason: "+failure.WaitingReason)
	}
	if failure.TerminatedReason != "" {
		evidence = append(evidence, "Termination reason: "+failure.TerminatedReason)
	}
	if failure.ExitCode != 0 {
		evidence = append(evidence, "Exit code: "+itoa32(failure.ExitCode))
	}
	if failure.LastTerminationReason != "" {
		evidence = append(evidence, "Last termination: "+failure.LastTerminationReason)
	}
	if failure.LastExitCode != 0 {
		evidence = append(evidence, "Last exit code: "+itoa32(failure.LastExitCode))
	}
	if failure.MemoryLimit != "" {
		evidence = append(evidence, "Memory limit: "+failure.MemoryLimit)
	}
	if failure.Message != "" {
		evidence = append(evidence, "Kubernetes message: "+failure.Message)
	}

	for _, event := range failure.Events {
		lower := strings.ToLower(event)
		switch failureType {
		case "ImagePullBackOff":
			if strings.Contains(lower, "manifest unknown") || strings.Contains(lower, "not found") {
				evidence = append(evidence, "Registry response indicates image/tag not found")
			}
			if strings.Contains(lower, "pull access denied") || strings.Contains(lower, "unauthorized") {
				evidence = append(evidence, "Registry denied pull request (auth/permissions)")
			}
		case "FailedScheduling":
			if strings.Contains(lower, "insufficient") || strings.Contains(lower, "taint") || strings.Contains(lower, "didn't match") {
				evidence = append(evidence, "Scheduler event: "+event)
			}
		case "ReadinessProbeFailed", "LivenessProbeFailed":
			if strings.Contains(lower, "probe failed") || strings.Contains(lower, "unhealthy") {
				evidence = append(evidence, "Probe event: "+event)
			}
		}
	}

	if len(failure.Events) > 0 {
		evidence = append(evidence, "Recent events captured: "+itoa(len(failure.Events)))
	}

	return uniqueStrings(evidence)
}

func buildContextSignals(failure k8s.PodFailure) []string {
	context := make([]string, 0, 2)
	if failure.RecentRollout {
		context = append(context, "Pod appears recently created (possible rollout impact)")
	}
	if failure.PodAgeSeconds > 0 {
		context = append(context, "Pod age: "+formatAge(failure.PodAgeSeconds))
	}
	return context
}

func deriveLikelyCause(defaultCause, failureType string, failure k8s.PodFailure, evidence []string) string {
	switch failureType {
	case "ImagePullBackOff":
		for _, event := range failure.Events {
			lower := strings.ToLower(event)
			if strings.Contains(lower, "manifest unknown") || strings.Contains(lower, "not found") {
				return "Image tag or repository does not exist in registry"
			}
			if strings.Contains(lower, "pull access denied") || strings.Contains(lower, "unauthorized") {
				return "Registry authentication or repository permission failure"
			}
		}
	case "CrashLoopBackOff":
		if failure.ExitCode != 0 || failure.LastExitCode != 0 {
			code := failure.ExitCode
			if code == 0 {
				code = failure.LastExitCode
			}
			return "Application repeatedly exits with non-zero code " + itoa32(code)
		}
		if failure.RestartCount >= 5 {
			return "Container repeatedly starts and crashes shortly after launch"
		}
	case "OOMKilled":
		if failure.MemoryLimit != "" {
			return "Container exceeded configured memory limit (" + failure.MemoryLimit + ")"
		}
		return "Container exceeded available memory and was terminated by kubelet"
	case "FailedScheduling":
		for _, line := range evidence {
			if strings.HasPrefix(line, "Scheduler event:") {
				return "Pod is unschedulable due to cluster capacity or placement constraints"
			}
		}
	case "ReadinessProbeFailed":
		return "Application is running but failing readiness checks"
	case "LivenessProbeFailed":
		return "Liveness checks are failing, causing container restarts"
	}

	return defaultCause
}

func deriveSuggestedFix(defaultFix, failureType string, failure k8s.PodFailure) string {
	namespace := failure.Namespace
	name := failure.Name

	switch failureType {
	case "ImagePullBackOff":
		if failure.Image != "" {
			return "Validate image name and tag. Try docker pull " + failure.Image + ". Then verify imagePullSecrets and registry permissions."
		}
		return defaultFix
	case "CrashLoopBackOff":
		if failure.Container != "" {
			return "Inspect previous logs with kubectl -n " + namespace + " logs " + name + " -c " + failure.Container + " --previous. Validate startup command and required environment variables."
		}
		return "Inspect previous logs with kubectl -n " + namespace + " logs " + name + " --previous. Validate startup command and required environment variables."
	case "OOMKilled":
		if failure.MemoryLimit != "" {
			return "Increase memory limit above " + failure.MemoryLimit + " and review memory usage profile for leaks or spikes."
		}
		return defaultFix
	case "FailedScheduling":
		return "Run kubectl -n " + namespace + " describe pod " + name + " to inspect scheduling errors. Scale nodes, relax resource requests, or adjust taints/tolerations."
	case "ReadinessProbeFailed", "LivenessProbeFailed":
		return "Check probe endpoint and timing settings. Confirm the app is listening on the expected port/path and tune initialDelaySeconds/timeoutSeconds."
	}

	return defaultFix
}

func buildQuickCommands(failureType string, failure k8s.PodFailure) []string {
	ns := failure.Namespace
	pod := failure.Name
	commands := []string{
		"kubectl -n " + ns + " describe pod " + pod,
		"kubectl -n " + ns + " get events --field-selector involvedObject.kind=Pod,involvedObject.name=" + pod + " --sort-by=.lastTimestamp",
	}

	switch failureType {
	case "CrashLoopBackOff", "OOMKilled", "ReadinessProbeFailed", "LivenessProbeFailed":
		if failure.Container != "" {
			commands = append(commands, "kubectl -n "+ns+" logs "+pod+" -c "+failure.Container+" --previous")
		} else {
			commands = append(commands, "kubectl -n "+ns+" logs "+pod+" --previous")
		}
	case "ImagePullBackOff":
		if failure.Image != "" {
			commands = append(commands, "docker pull "+failure.Image)
		}
		commands = append(commands, "kubectl -n "+ns+" get pod "+pod+" -o jsonpath='{.spec.imagePullSecrets}'")
	case "FailedScheduling":
		commands = append(commands, "kubectl describe nodes")
	}

	return uniqueStrings(commands)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func formatAge(seconds int64) string {
	if seconds < 60 {
		return itoa(int(seconds)) + "s"
	}
	if seconds < 3600 {
		return itoa(int(seconds/60)) + "m"
	}
	return itoa(int(seconds/3600)) + "h"
}

func itoa32(v int32) string {
	return itoa(int(v))
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	negative := v < 0
	if negative {
		v = -v
	}
	buf := make([]byte, 0, 12)
	for v > 0 {
		d := v % 10
		buf = append([]byte{byte('0' + d)}, buf...)
		v /= 10
	}
	if negative {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
