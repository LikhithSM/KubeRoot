package analyzer

import (
	"strings"
	"time"

	"kuberoot/internal/k8s"
)

type Diagnosis struct {
	OrganizationID string          `json:"organizationId"`
	ClusterID      string          `json:"clusterId"`
	PodName        string          `json:"podName"`
	Namespace      string          `json:"namespace"`
	Container      string          `json:"container"`
	Image          string          `json:"image"`
	RestartCount   int32           `json:"restartCount"`
	FailureType    string          `json:"failureType"`
	Category       string          `json:"category"`
	Severity       string          `json:"severity"` // critical | high | medium | low
	LikelyCause    string          `json:"likelyCause"`
	SuggestedFix   string          `json:"suggestedFix"`
	Confidence     string          `json:"confidence"`
	ConfidenceNote string          `json:"confidenceNote"`
	Evidence       []string        `json:"evidence"`
	FixSuggestions []FixSuggestion `json:"fixSuggestions"`
	QuickCommands  []string        `json:"quickCommands"`
	Context        []string        `json:"context"`
	Events         []string        `json:"events"`
	Timestamp      time.Time       `json:"timestamp"`
}

type FixSuggestion struct {
	Title       string `json:"title"`
	Explanation string `json:"explanation"`
	Command     string `json:"command"`
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
	{
		FailureType:  "DNSLookupFailed",
		LikelyCause:  "Application failed to resolve a dependency hostname",
		SuggestedFix: "Validate DNS name and service existence",
		Confidence:   "high",
	},
	{
		FailureType:  "ImageRegistryDNSFailure",
		LikelyCause:  "Node/container runtime failed to resolve registry hostname while pulling image",
		SuggestedFix: "Verify registry DNS host and node DNS/proxy configuration",
		Confidence:   "high",
	},
	{
		FailureType:  "NetworkTimeout",
		LikelyCause:  "Application timed out reaching a dependency",
		SuggestedFix: "Check service endpoints, network policies, and destination availability",
		Confidence:   "medium",
	},
	{
		FailureType:  "DeploymentRolloutFailed",
		LikelyCause:  "Deployment rollout did not make progress before the progress deadline",
		SuggestedFix: "Inspect deployment rollout status, recent spec changes, and failing pod diagnostics",
		Confidence:   "high",
	},
}

func DiagnoseFailures(orgID, clusterID string, failures []k8s.PodFailure) []Diagnosis {
	engine := NewDiagnosisEngine(v1Rules)

	out := make([]Diagnosis, 0, len(failures))
	for _, failure := range failures {
		for _, failureType := range failure.Types {
			diagnosis, ok := engine.Diagnose(orgID, clusterID, failure, failureType)
			if !ok {
				continue
			}
			out = append(out, diagnosis)
		}
	}

	return out
}

func HydrateDiagnosis(d *Diagnosis) {
	if d == nil {
		return
	}

	failure := k8s.PodFailure{
		Namespace:    d.Namespace,
		Name:         d.PodName,
		Container:    d.Container,
		Image:        d.Image,
		RestartCount: d.RestartCount,
		Events:       d.Events,
	}

	if strings.TrimSpace(d.Category) == "" {
		d.Category = categorizeFailure(d.FailureType)
	}
	if strings.TrimSpace(d.Severity) == "" {
		d.Severity = computeSeverity(d.Confidence, d.FailureType, failure)
	}
	if len(d.FixSuggestions) == 0 {
		d.FixSuggestions = buildFixSuggestions(d.FailureType, failure, d.Evidence)
	}
	d.FixSuggestions = sanitizeFixSuggestions(d.FixSuggestions)
	if strings.TrimSpace(d.SuggestedFix) == "" || strings.Contains(d.SuggestedFix, "Inspect") || strings.Contains(d.SuggestedFix, "Verify") || strings.Contains(d.SuggestedFix, "Check") {
		d.SuggestedFix = deriveSuggestedFix(d.SuggestedFix, d.FailureType, failure, d.Evidence, d.FixSuggestions)
	}
}

func sanitizeFixSuggestions(fixes []FixSuggestion) []FixSuggestion {
	if len(fixes) == 0 {
		return nil
	}

	clean := make([]FixSuggestion, 0, len(fixes))
	for _, fix := range fixes {
		cmd := strings.TrimSpace(fix.Command)
		title := strings.TrimSpace(fix.Title)
		explanation := strings.TrimSpace(fix.Explanation)

		if cmd == "" && title == "" && explanation == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(cmd), "common causes:") {
			continue
		}

		clean = append(clean, FixSuggestion{
			Title:       title,
			Explanation: explanation,
			Command:     cmd,
		})
	}

	return clean
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
		case "DNSLookupFailed":
			if strings.Contains(lowerEvent, "lookup") && strings.Contains(lowerEvent, "no such host") {
				evidenceScore = maxInt(evidenceScore, 1)
				reasons = append(reasons, "dns lookup failure observed in events")
			}
		case "ImageRegistryDNSFailure":
			if strings.Contains(lowerEvent, "lookup") && strings.Contains(lowerEvent, "no such host") {
				evidenceScore = maxInt(evidenceScore, 1)
				reasons = append(reasons, "registry dns lookup failure observed during image pull")
			}
		case "NetworkTimeout":
			if strings.Contains(lowerEvent, "i/o timeout") || strings.Contains(lowerEvent, "connection timed out") || strings.Contains(lowerEvent, "context deadline exceeded") {
				evidenceScore = maxInt(evidenceScore, 1)
				reasons = append(reasons, "network timeout observed in events")
			}
		case "DeploymentRolloutFailed":
			if strings.Contains(lowerEvent, "progress deadline exceeded") || strings.Contains(lowerEvent, "timed out progressing") {
				evidenceScore = maxInt(evidenceScore, 1)
				reasons = append(reasons, "deployment rollout timeout reported by controller")
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
	case "DNSLookupFailed", "NetworkTimeout":
		score += 1
	case "ImageRegistryDNSFailure":
		score += 1
	case "DeploymentRolloutFailed":
		score += 2
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
	if failure.Deployment != "" {
		evidence = append(evidence, "Deployment: "+failure.Deployment)
	}
	if failure.DeploymentRevision != "" {
		evidence = append(evidence, "Deployment revision: "+failure.DeploymentRevision)
	}
	if failure.ContainerCommand != "" {
		evidence = append(evidence, "Container command: "+failure.ContainerCommand)
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
		case "DeploymentRolloutFailed":
			if strings.Contains(lower, "progress deadline exceeded") || strings.Contains(lower, "timed out progressing") {
				evidence = append(evidence, "Rollout timeout event: "+event)
			}
		case "ImageRegistryDNSFailure":
			if strings.Contains(lower, "lookup") && strings.Contains(lower, "no such host") {
				evidence = append(evidence, "Registry DNS resolution failure: "+event)
			}
		}

		if strings.Contains(lower, "lookup") && strings.Contains(lower, "no such host") {
			evidence = append(evidence, "DNS lookup failure: "+event)
		}
		if strings.Contains(lower, "connection refused") || strings.Contains(lower, "econnrefused") {
			evidence = append(evidence, "Connection refused: "+event)
		}
		if strings.Contains(lower, "i/o timeout") || strings.Contains(lower, "connection timed out") || strings.Contains(lower, "context deadline exceeded") {
			evidence = append(evidence, "Network timeout: "+event)
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
	context := make([]string, 0, 8)
	if failure.RecentRollout {
		context = append(context, "Pod appears recently created (possible rollout impact)")
	}
	if failure.PodAgeSeconds > 0 {
		context = append(context, "Pod age: "+formatAge(failure.PodAgeSeconds))
	}
	if failure.Deployment != "" {
		context = append(context, "Deployment: "+failure.Deployment)
	}
	if failure.DeploymentRevision != "" {
		context = append(context, "Deployment revision: "+failure.DeploymentRevision)
	}
	if failure.ReplicaStatus != "" {
		context = append(context, "Replicas: "+failure.ReplicaStatus+" ready")
	}
	if len(failure.Services) > 0 {
		context = append(context, "Services: "+strings.Join(failure.Services, ", "))
	}
	if len(failure.ConfigMaps) > 0 {
		context = append(context, "ConfigMaps: "+strings.Join(failure.ConfigMaps, ", "))
	}
	if len(failure.Secrets) > 0 {
		context = append(context, "Secrets: "+strings.Join(failure.Secrets, ", "))
	}
	if len(failure.EnvVariables) > 0 {
		maxEnv := len(failure.EnvVariables)
		if maxEnv > 8 {
			maxEnv = 8
		}
		context = append(context, "Env vars: "+strings.Join(failure.EnvVariables[:maxEnv], ", "))
	}
	if failure.ContainerCommand != "" {
		context = append(context, "Command: "+failure.ContainerCommand)
	}
	return context
}

func deriveLikelyCause(defaultCause, failureType string, failure k8s.PodFailure, evidence []string) string {
	switch failureType {
	case "ImagePullBackOff":
		for _, e := range evidence {
			if strings.Contains(e, "not found") {
				if failure.Image != "" {
					return "Image " + failure.Image + " does not exist in the registry"
				}
				return "Image tag or repository does not exist in registry"
			}
			if strings.Contains(e, "access denied") || strings.Contains(e, "insufficient_scope") {
				return "Missing registry credentials or repository permission for image pull"
			}
		}
	case "ImageRegistryDNSFailure":
		if failure.Image != "" {
			return "Container runtime could not resolve registry host while pulling image " + failure.Image
		}
		return "Container runtime could not resolve image registry hostname during pull"
	case "CrashLoopBackOff":
		combined := strings.ToLower(strings.Join(append([]string{failure.Message}, failure.Events...), "\n"))
		if strings.Contains(combined, "econnrefused") || strings.Contains(combined, "connection refused") {
			if len(failure.Services) > 0 {
				return "Application cannot connect to service " + failure.Services[0] + " (connection refused)"
			}
			return "Application cannot connect to dependency (connection refused)"
		}
		if (strings.Contains(combined, "lookup") && strings.Contains(combined, "no such host")) || strings.Contains(combined, "temporary failure in name resolution") {
			return "Application failed DNS lookup for a dependent service"
		}
		if strings.Contains(combined, "i/o timeout") || strings.Contains(combined, "connection timed out") || strings.Contains(combined, "context deadline exceeded") {
			return "Application cannot reach dependency due to network timeout"
		}
		if strings.Contains(combined, "permission denied") && failure.ContainerCommand != "" {
			return "Container command failed with permission denied (check executable path/permissions): " + failure.ContainerCommand
		}
		if failure.ExitCode != 0 || failure.LastExitCode != 0 {
			code := failure.ExitCode
			if code == 0 {
				code = failure.LastExitCode
			}
			if code == 137 {
				if failure.MemoryLimit != "" {
					return "Process terminated with exit code 137 (likely memory pressure near limit " + failure.MemoryLimit + ")"
				}
				return "Process terminated with exit code 137 (likely memory pressure or forced kill)"
			}
			if code == 127 {
				return "Process exited with code 127 (startup command or binary not found)"
			}
			if code == 126 {
				return "Process exited with code 126 (startup command found but not executable)"
			}
			if code == 1 && failure.RecentRollout {
				if failure.DeploymentRevision != "" {
					return "Application began crashing right after rollout revision " + failure.DeploymentRevision + " (exit code 1)"
				}
				return "Application began crashing immediately after a recent rollout (exit code 1)"
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
	case "DNSLookupFailed":
		return "Application failed DNS resolution for a service/dependency hostname"
	case "NetworkTimeout":
		return "Application timed out while connecting to a dependency endpoint"
	case "DeploymentRolloutFailed":
		if failure.Deployment != "" && failure.ReplicaStatus != "" {
			return "Deployment " + failure.Deployment + " rollout stalled with only " + failure.ReplicaStatus + " replicas ready before progress deadline"
		}
		if failure.Deployment != "" {
			return "Deployment " + failure.Deployment + " rollout stalled before progress deadline"
		}
		return "Deployment rollout failed to progress before progress deadline"
	}

	return defaultCause
}

func deriveSuggestedFix(defaultFix, failureType string, failure k8s.PodFailure, evidence []string, fixSuggestions []FixSuggestion) string {
	if len(fixSuggestions) > 0 {
		primary := fixSuggestions[0]
		if primary.Command != "" {
			return primary.Explanation + "\n\n" + primary.Command
		}
		if primary.Explanation != "" {
			return primary.Explanation
		}
	}

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
	case "DNSLookupFailed":
		return "1. Verify the dependency hostname exists as a Kubernetes Service\n2. Check DNS suffix and namespace (svc.cluster.local)\n3. Validate CoreDNS health and pod DNS config"
	case "ImageRegistryDNSFailure":
		return "1. Verify registry hostname in image reference\n2. Validate node DNS can resolve the registry host\n3. Check proxy/firewall egress to registry"
	case "NetworkTimeout":
		return "1. Check endpoints for the target Service\n2. Validate NetworkPolicies allow traffic\n3. Ensure destination pods are healthy and listening"
	case "DeploymentRolloutFailed":
		if failure.Deployment != "" {
			return "1. Check rollout status: kubectl -n " + ns + " rollout status deployment/" + failure.Deployment + "\n2. Inspect deployment events: kubectl -n " + ns + " describe deployment " + failure.Deployment + "\n3. Compare current revision to previous and inspect failing pod logs"
		}
		return "1. Inspect deployment events for progress deadline failures\n2. Compare recent image/config changes\n3. Check logs of new pods created during rollout"
	}

	return defaultFix
}

func buildFixSuggestions(failureType string, failure k8s.PodFailure, evidence []string) []FixSuggestion {
	ns := failure.Namespace
	pod := failure.Name
	image := failure.Image

	switch failureType {
	case "ImagePullBackOff":
		fixes := make([]FixSuggestion, 0, 3)
		fixes = append(fixes, FixSuggestion{
			Title:       "Check deployment image",
			Explanation: "Confirm the workload is using the image you expect before patching credentials or tags.",
			Command:     "kubectl -n " + ns + " get deployment -o yaml | grep image",
		})
		for _, e := range evidence {
			if strings.Contains(e, "not found") {
				fixes = append(fixes,
					FixSuggestion{
						Title:       "Use an existing image tag",
						Explanation: "Update the Deployment to an image tag that already exists in the registry.",
						Command:     "image: nginx:latest",
					},
					FixSuggestion{
						Title:       "Push the missing image",
						Explanation: "If this repository/tag should exist, publish it before the rollout continues.",
						Command:     "docker push " + defaultValue(image, "<registry>/<image>:<tag>"),
					},
				)
				return fixes
			}
			if strings.Contains(e, "access denied") {
				fixes = append(fixes,
					FixSuggestion{
						Title:       "Add registry credentials",
						Explanation: "Create a Docker registry secret in the failing namespace.",
						Command:     "kubectl create secret docker-registry regcred \\\n  --docker-server=docker.io \\\n  --docker-username=<user> \\\n  --docker-password=<password> \\\n  -n " + ns,
					},
					FixSuggestion{
						Title:       "Attach imagePullSecrets",
						Explanation: "Patch the Deployment so kubelet uses the registry credentials during image pull.",
						Command:     "spec:\n  imagePullSecrets:\n  - name: regcred",
					},
				)
				return fixes
			}
		}
		return fixes
	case "ConfigMapMissing":
		name := missingResourceName(evidence, "ConfigMap not found: ")
		if name == "" {
			name = "app-config"
		}
		return []FixSuggestion{
			{
				Title:       "Create the missing ConfigMap",
				Explanation: "Create the ConfigMap that the Deployment is already referencing.",
				Command:     "kubectl create configmap " + name + " \\\n  --from-env-file=config.env \\\n  -n " + ns,
			},
			{
				Title:       "Verify the reference name",
				Explanation: "Check the Deployment manifest for the referenced ConfigMap name.",
				Command:     "kubectl -n " + ns + " get deployment -o yaml | grep -A3 " + name,
			},
		}
	case "SecretMissing":
		name := missingResourceName(evidence, "Secret not found: ")
		if name == "" {
			name = "db-secret"
		}
		return []FixSuggestion{
			{
				Title:       "Create the missing Secret",
				Explanation: "Create the Secret that the Deployment expects in this namespace.",
				Command:     "kubectl create secret generic " + name + " \\\n  --from-literal=password=<value> \\\n  -n " + ns,
			},
			{
				Title:       "Verify the secret reference",
				Explanation: "Check the Deployment manifest for the referenced Secret name.",
				Command:     "kubectl -n " + ns + " get deployment -o yaml | grep -A3 " + name,
			},
		}
	case "CrashLoopBackOff":
		logsCmd := "kubectl -n " + ns + " logs " + pod + " --previous"
		if failure.Container != "" {
			logsCmd = "kubectl -n " + ns + " logs " + pod + " -c " + failure.Container + " --previous"
		}
		describeCmd := "kubectl -n " + ns + " describe pod " + pod
		if failure.Container != "" {
			describeCmd = "kubectl -n " + ns + " get pod " + pod + " -o jsonpath='{.spec.containers[?(@.name==\"" + failure.Container + "\")].command}'"
		}
		return []FixSuggestion{
			{
				Title:       "Inspect previous logs",
				Explanation: "Start with the last crashed container logs. This usually exposes the exact startup error.",
				Command:     logsCmd,
			},
			{
				Title:       "Check effective startup command",
				Explanation: "Verify the container command/args running in the pod are what your app expects.",
				Command:     describeCmd,
			},
		}
	case "OOMKilled":
		return []FixSuggestion{
			{
				Title:       "Increase memory limit",
				Explanation: "Raise the memory limit above the current ceiling and redeploy.",
				Command:     "resources:\n  limits:\n    memory: 512Mi",
			},
			{
				Title:       "Confirm runtime memory usage",
				Explanation: "Check whether the container is genuinely exceeding its limit before increasing it again.",
				Command:     "kubectl top pod " + pod + " -n " + ns,
			},
		}
	case "FailedScheduling":
		return []FixSuggestion{
			{
				Title:       "Inspect node capacity",
				Explanation: "Confirm whether the cluster has enough allocatable CPU and memory.",
				Command:     "kubectl get nodes -o custom-columns=NAME:.metadata.name,CPU:.status.allocatable.cpu,MEMORY:.status.allocatable.memory",
			},
			{
				Title:       "Reduce resource requests",
				Explanation: "If the pod is over-requesting resources, lower requests so it can schedule.",
				Command:     "resources:\n  requests:\n    cpu: 100m\n    memory: 128Mi",
			},
		}
	case "ReadinessProbeFailed", "LivenessProbeFailed":
		probeName := "readinessProbe"
		if failureType == "LivenessProbeFailed" {
			probeName = "livenessProbe"
		}
		return []FixSuggestion{
			{
				Title:       "Check probe config",
				Explanation: "Verify the probe path, port, and timing in the Deployment manifest.",
				Command:     "kubectl -n " + ns + " get deployment -o yaml | grep -A10 " + probeName,
			},
			{
				Title:       "Delay probe startup",
				Explanation: "If the application starts slowly, increase the initial delay before probes begin.",
				Command:     probeName + ":\n  initialDelaySeconds: 20\n  timeoutSeconds: 2",
			},
		}
	case "PodPending":
		return []FixSuggestion{
			{
				Title:       "Describe the pending pod",
				Explanation: "The describe output will tell you whether the block is scheduling, volume, or image related.",
				Command:     "kubectl -n " + ns + " describe pod " + pod,
			},
		}
	case "DNSLookupFailed":
		return []FixSuggestion{
			{
				Title:       "Verify service DNS target",
				Explanation: "Confirm the service exists and has endpoints in the expected namespace.",
				Command:     "kubectl -n " + ns + " get svc && kubectl -n " + ns + " get endpoints",
			},
			{
				Title:       "Use FQDN in environment",
				Explanation: "Set dependency host to the full Kubernetes DNS name.",
				Command:     "DATABASE_HOST=postgres.default.svc.cluster.local",
			},
		}
	case "ImageRegistryDNSFailure":
		return []FixSuggestion{
			{
				Title:       "Verify image registry hostname",
				Explanation: "Ensure the image reference uses a valid and resolvable registry host.",
				Command:     "kubectl -n " + ns + " describe pod " + pod,
			},
			{
				Title:       "Check DNS from a cluster node",
				Explanation: "Confirm cluster/node DNS can resolve the registry domain used in the image.",
				Command:     "nslookup <registry-hostname>",
			},
		}
	case "NetworkTimeout":
		return []FixSuggestion{
			{
				Title:       "Check service endpoints",
				Explanation: "Ensure the destination service has ready pod endpoints.",
				Command:     "kubectl -n " + ns + " get endpoints",
			},
			{
				Title:       "Inspect network policies",
				Explanation: "Verify no NetworkPolicy is blocking traffic between source and destination.",
				Command:     "kubectl -n " + ns + " get networkpolicy",
			},
		}
	case "DeploymentRolloutFailed":
		target := failure.Deployment
		if strings.TrimSpace(target) == "" {
			target = "<deployment-name>"
		}
		return []FixSuggestion{
			{
				Title:       "Inspect rollout status",
				Explanation: "Confirm exactly which condition is blocking rollout progress.",
				Command:     "kubectl -n " + ns + " rollout status deployment/" + target,
			},
			{
				Title:       "Review deployment change",
				Explanation: "Compare image, command, env, and config references introduced in the current revision.",
				Command:     "kubectl -n " + ns + " describe deployment " + target,
			},
			{
				Title:       "Rollback if customer impact is high",
				Explanation: "If the new revision is unhealthy, rollback to the previous working revision to stop the incident.",
				Command:     "kubectl -n " + ns + " rollout undo deployment/" + target,
			},
		}
	}

	return nil
}

func missingResourceName(evidence []string, prefix string) string {
	for _, e := range evidence {
		if strings.HasPrefix(e, prefix) {
			return strings.TrimPrefix(e, prefix)
		}
	}
	return ""
}

func categorizeFailure(failureType string) string {
	switch failureType {
	case "ImagePullBackOff":
		return "Registry error"
	case "ConfigMapMissing", "SecretMissing":
		return "Configuration error"
	case "CrashLoopBackOff":
		return "Application startup"
	case "OOMKilled", "FailedScheduling", "PodPending":
		return "Resource constraint"
	case "DNSLookupFailed", "NetworkTimeout":
		return "Connectivity"
	case "ImageRegistryDNSFailure":
		return "Registry error"
	case "DeploymentRolloutFailed":
		return "Rollout"
	case "ReadinessProbeFailed", "LivenessProbeFailed":
		return "Health check"
	default:
		return "Runtime issue"
	}
}

func defaultValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
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
	case "DNSLookupFailed":
		commands = append(commands,
			"kubectl -n "+ns+" get svc",
			"kubectl -n "+ns+" get endpoints",
		)
	case "ImageRegistryDNSFailure":
		commands = append(commands,
			"kubectl -n "+ns+" describe pod "+pod,
			"nslookup <registry-hostname>",
		)
	case "NetworkTimeout":
		commands = append(commands,
			"kubectl -n "+ns+" get endpoints",
			"kubectl -n "+ns+" get networkpolicy",
		)
	case "DeploymentRolloutFailed":
		target := failure.Deployment
		if strings.TrimSpace(target) == "" {
			target = "<deployment-name>"
		}
		commands = append(commands,
			"kubectl -n "+ns+" rollout status deployment/"+target,
			"kubectl -n "+ns+" describe deployment "+target,
			"kubectl -n "+ns+" rollout history deployment/"+target,
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
