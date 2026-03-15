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
	Severity       string    `json:"severity"` // critical | high | medium | low
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
		SuggestedFix: "Verify image name and tag; check imagePullSecrets and registry permissions",
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
	{
		FailureType:  "ConfigMapMissing",
		LikelyCause:  "Pod references a ConfigMap that does not exist",
		SuggestedFix: "Create the missing ConfigMap or fix the reference in the Deployment",
		Confidence:   "high",
	},
	{
		FailureType:  "SecretMissing",
		LikelyCause:  "Pod references a Secret that does not exist",
		SuggestedFix: "Create the missing Secret or fix the reference in the Deployment",
		Confidence:   "high",
	},
	{
		FailureType:  "PodPending",
		LikelyCause:  "Pod is stuck pending — likely a missing volume, ConfigMap, or resource constraint",
		SuggestedFix: "Check pod events for mount failures or scheduling issues",
		Confidence:   "low",
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
			ctx := buildContextSignals(failure)
			likelyCause := deriveLikelyCause(rule.LikelyCause, failureType, failure, evidence)
			suggestedFix := deriveSuggestedFix(rule.SuggestedFix, failureType, failure, evidence)
			quickCommands := buildQuickCommands(failureType, failure, evidence)
			confidence, confidenceNote := enrichConfidence(rule.Confidence, failureType, failure, evidence)
			severity := computeSeverity(confidence, failureType, failure)

			out = append(out, Diagnosis{
				OrganizationID: orgID,
				ClusterID:      clusterID,
				PodName:        failure.Name,
				Namespace:      failure.Namespace,
				Container:      failure.Container,
				Image:          failure.Image,
				RestartCount:   failure.RestartCount,
				FailureType:    rule.FailureType,
				Severity:       severity,
				LikelyCause:    likelyCause,
				SuggestedFix:   suggestedFix,
				Confidence:     confidence,
				ConfidenceNote: confidenceNote,
				Evidence:       evidence,
				QuickCommands:  quickCommands,
				Context:        ctx,
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
		case "ConfigMapMissing", "SecretMissing":
			if strings.Contains(lowerEvent, "not found") && (strings.Contains(lowerEvent, "configmap") || strings.Contains(lowerEvent, "secret")) {
				evidenceScore = maxInt(evidenceScore, 1)
				reasons = append(reasons, "mount failure event names missing resource")
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

// ---------------------------------------------------------------------------
// Severity scoring
// ---------------------------------------------------------------------------

func computeSeverity(confidence, failureType string, failure k8s.PodFailure) string {
	score := 0
	switch confidence {
	case "high":
		score += 3
	case "medium":
		score += 2
	default:
		score += 1
	}
	switch failureType {
	case "OOMKilled", "CrashLoopBackOff":
		score += 1
	case "ConfigMapMissing", "SecretMissing":
		score += 1
	}
	if failure.RestartCount >= 10 {
		score += 2
	} else if failure.RestartCount >= 3 {
		score += 1
	}
	switch {
	case score >= 6:
		return "critical"
	case score >= 4:
		return "high"
	case score >= 3:
		return "medium"
	default:
		return "low"
	}
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
			if strings.Contains(lower, "manifest unknown") || (strings.Contains(lower, "not found") && strings.Contains(lower, "image")) {
				evidence = append(evidence, "Registry response: image or tag not found")
			}
			if strings.Contains(lower, "pull access denied") || strings.Contains(lower, "insufficient_scope") || strings.Contains(lower, "unauthorized") {
				evidence = append(evidence, "Registry response: access denied (check imagePullSecrets)")
			}
			if name := extractQuoted(event, "pulling image"); name != "" {
				evidence = append(evidence, "Pull attempt: "+name)
			}
		case "FailedScheduling":
			if strings.Contains(lower, "insufficient cpu") {
				evidence = append(evidence, "Scheduler: insufficient CPU on all nodes")
			}
			if strings.Contains(lower, "insufficient memory") {
				evidence = append(evidence, "Scheduler: insufficient memory on all nodes")
			}
			if strings.Contains(lower, "0/") && strings.Contains(lower, "nodes available") {
				evidence = append(evidence, "Scheduler event: "+event)
			}
			if strings.Contains(lower, "taint") {
				evidence = append(evidence, "Node taint mismatch detected")
			}
		case "ReadinessProbeFailed", "LivenessProbeFailed":
			if strings.Contains(lower, "probe failed") || strings.Contains(lower, "unhealthy") {
				evidence = append(evidence, "Probe event: "+event)
			}
		case "OOMKilled":
			if strings.Contains(lower, "oomkilled") || strings.Contains(lower, "out of memory") {
				evidence = append(evidence, "Kernel OOM event observed")
			}
		case "ConfigMapMissing":
			if name := extractQuoted(event, "configmap"); name != "" {
				evidence = append(evidence, "ConfigMap not found: "+name)
			}
			if strings.Contains(lower, "mountvolume") || strings.Contains(lower, "mount failed") {
				evidence = append(evidence, "Volume mount failure: "+event)
			}
		case "SecretMissing":
			if name := extractQuoted(event, "secret"); name != "" {
				evidence = append(evidence, "Secret not found: "+name)
			}
			if strings.Contains(lower, "mountvolume") || strings.Contains(lower, "mount failed") {
				evidence = append(evidence, "Volume mount failure: "+event)
			}
		}
	}

	if len(failure.Events) > 0 {
		evidence = append(evidence, "Pod events captured: "+itoa(len(failure.Events)))
	}

	return uniqueStrings(evidence)
}

// extractQuoted extracts the first quoted string after a keyword:
// e.g. extractQuoted(`configmap "payment-config" not found`, "configmap") → "payment-config"
func extractQuoted(text, keyword string) string {
	lower := strings.ToLower(text)
	kw := strings.ToLower(keyword) + " \""
	idx := strings.Index(lower, kw)
	if idx < 0 {
		return ""
	}
	start := idx + len(kw)
	end := strings.Index(text[start:], "\"")
	if end < 0 {
		return ""
	}
	return text[start : start+end]
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
		for _, e := range evidence {
			if strings.Contains(e, "not found") {
				if failure.Image != "" {
					return "Image tag or repository does not exist: " + failure.Image
				}
				return "Image tag or repository does not exist in registry"
			}
			if strings.Contains(e, "access denied") || strings.Contains(e, "insufficient_scope") {
				return "Registry authentication or repository permission failure"
			}
		}
	case "CrashLoopBackOff":
		if failure.ExitCode != 0 || failure.LastExitCode != 0 {
			code := failure.ExitCode
			if code == 0 {
				code = failure.LastExitCode
			}
			return "Application exited with non-zero code " + itoa32(code) + " — likely startup error or misconfiguration"
		}
		if failure.RestartCount >= 5 {
			return "Container continuously crashing shortly after launch (" + itoa32(failure.RestartCount) + " restarts)"
		}
	case "OOMKilled":
		if failure.MemoryLimit != "" {
			return "Container terminated by kernel OOM killer — exceeded memory limit " + failure.MemoryLimit
		}
		return "Container consumed more memory than allowed and was killed"
	case "FailedScheduling":
		for _, e := range evidence {
			if strings.Contains(e, "insufficient CPU") {
				return "Cluster has no nodes with sufficient CPU to schedule this pod"
			}
			if strings.Contains(e, "insufficient memory") {
				return "Cluster has no nodes with sufficient memory to schedule this pod"
			}
			if strings.Contains(e, "taint") {
				return "Pod does not tolerate node taints — no eligible nodes found"
			}
		}
		return "Pod cannot be scheduled — cluster capacity or placement constraint"
	case "ReadinessProbeFailed":
		return "Application started but is not passing readiness checks — traffic is being withheld"
	case "LivenessProbeFailed":
		return "Liveness probe is failing — kubelet will restart the container"
	case "ConfigMapMissing":
		for _, e := range evidence {
			if strings.HasPrefix(e, "ConfigMap not found: ") {
				name := strings.TrimPrefix(e, "ConfigMap not found: ")
				return "Deployment references ConfigMap \"" + name + "\" which does not exist in namespace " + failure.Namespace
			}
		}
		return "Deployment references a ConfigMap that does not exist in namespace " + failure.Namespace
	case "SecretMissing":
		for _, e := range evidence {
			if strings.HasPrefix(e, "Secret not found: ") {
				name := strings.TrimPrefix(e, "Secret not found: ")
				return "Deployment references Secret \"" + name + "\" which does not exist in namespace " + failure.Namespace
			}
		}
		return "Deployment references a Secret that does not exist in namespace " + failure.Namespace
	case "PodPending":
		return "Pod is stuck in pending state — waiting for volume mounts, scheduling, or resource availability"
	}

	return defaultCause
}

func deriveSuggestedFix(defaultFix, failureType string, failure k8s.PodFailure, evidence []string) string {
	ns := failure.Namespace
	pod := failure.Name

	switch failureType {
	case "ImagePullBackOff":
		if failure.Image != "" {
			return "Run: docker pull " + failure.Image + "\n\nIf the image is private, verify imagePullSecrets in the Deployment spec. If the tag is wrong, update the image field."
		}
		return "Verify image name and tag. Check imagePullSecrets. Run: kubectl -n " + ns + " get pod " + pod + " -o jsonpath='{.spec.imagePullSecrets}'"
	case "CrashLoopBackOff":
		if failure.Container != "" {
			return "Check crash logs:\nkubectl -n " + ns + " logs " + pod + " -c " + failure.Container + " --previous\n\nLook for: missing env vars, failed DB connections, bad startup config."
		}
		return "Check crash logs:\nkubectl -n " + ns + " logs " + pod + " --previous\n\nLook for: missing env vars, failed DB connections, bad startup config."
	case "OOMKilled":
		if failure.MemoryLimit != "" {
			return "Increase memory limit above " + failure.MemoryLimit + " in the container spec:\n\nresources:\n  limits:\n    memory: 256Mi\n\nThen investigate memory usage to find leaks."
		}
		return "Add or increase resources.limits.memory in the container spec."
	case "FailedScheduling":
		return "1. kubectl describe nodes — check CPU/memory available\n2. kubectl -n " + ns + " describe pod " + pod + " — inspect scheduling message\n3. Reduce resource requests or add cluster nodes."
	case "ReadinessProbeFailed", "LivenessProbeFailed":
		return "1. Confirm the probe path/port is correct in the Deployment spec\n2. Check the app is ready to serve before probes fire (tune initialDelaySeconds)\n3. kubectl -n " + ns + " logs " + pod + " to see health endpoint errors"
	case "ConfigMapMissing":
		for _, e := range evidence {
			if strings.HasPrefix(e, "ConfigMap not found: ") {
				name := strings.TrimPrefix(e, "ConfigMap not found: ")
				return "Create the missing ConfigMap:\n\nkubectl create configmap " + name + " --from-env-file=config.env -n " + ns + "\n\nOR update the Deployment to reference an existing ConfigMap."
			}
		}
		return "Create the missing ConfigMap referenced in the Deployment volumes or envFrom section."
	case "SecretMissing":
		for _, e := range evidence {
			if strings.HasPrefix(e, "Secret not found: ") {
				name := strings.TrimPrefix(e, "Secret not found: ")
				return "Create the missing Secret:\n\nkubectl create secret generic " + name + " --from-literal=key=value -n " + ns + "\n\nOR update the Deployment to reference an existing Secret."
			}
		}
		return "Create the missing Secret referenced in the Deployment volumes or envFrom section."
	case "PodPending":
		return "Run: kubectl -n " + ns + " describe pod " + pod + "\n\nLook for volume mount errors, scheduling failures, or missing resources."
	}

	return defaultFix
}

func buildQuickCommands(failureType string, failure k8s.PodFailure, evidence []string) []string {
	ns := failure.Namespace
	pod := failure.Name
	commands := []string{
		"kubectl -n " + ns + " describe pod " + pod,
		"kubectl -n " + ns + " get events --field-selector involvedObject.name=" + pod + " --sort-by=.lastTimestamp",
	}

	switch failureType {
	case "CrashLoopBackOff", "OOMKilled", "ReadinessProbeFailed", "LivenessProbeFailed":
		if failure.Container != "" {
			commands = append(commands,
				"kubectl -n "+ns+" logs "+pod+" -c "+failure.Container+" --previous",
				"kubectl -n "+ns+" logs "+pod+" -c "+failure.Container,
			)
		} else {
			commands = append(commands,
				"kubectl -n "+ns+" logs "+pod+" --previous",
				"kubectl -n "+ns+" logs "+pod,
			)
		}
	case "ImagePullBackOff":
		if failure.Image != "" {
			commands = append(commands, "docker pull "+failure.Image)
		}
		commands = append(commands, "kubectl -n "+ns+" get pod "+pod+" -o jsonpath='{.spec.imagePullSecrets}'")
	case "FailedScheduling":
		commands = append(commands,
			"kubectl describe nodes | grep -A5 'Conditions:'",
			"kubectl get nodes -o custom-columns=NAME:.metadata.name,CPU:.status.allocatable.cpu,MEMORY:.status.allocatable.memory",
		)
	case "ConfigMapMissing":
		for _, e := range evidence {
			if strings.HasPrefix(e, "ConfigMap not found: ") {
				name := strings.TrimPrefix(e, "ConfigMap not found: ")
				commands = append(commands,
					"kubectl -n "+ns+" get configmap "+name,
					"kubectl -n "+ns+" get configmaps",
				)
				break
			}
		}
		if len(commands) == 2 {
			commands = append(commands, "kubectl -n "+ns+" get configmaps")
		}
	case "SecretMissing":
		for _, e := range evidence {
			if strings.HasPrefix(e, "Secret not found: ") {
				name := strings.TrimPrefix(e, "Secret not found: ")
				commands = append(commands,
					"kubectl -n "+ns+" get secret "+name,
					"kubectl -n "+ns+" get secrets",
				)
				break
			}
		}
		if len(commands) == 2 {
			commands = append(commands, "kubectl -n "+ns+" get secrets")
		}
	case "PodPending":
		commands = append(commands, "kubectl -n "+ns+" get pod "+pod+" -o yaml")
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
