package analyzer

type staticFailureRule struct {
	name       string
	priority   int
	failureType string
}

func (r staticFailureRule) Name() string { return r.name }

func (r staticFailureRule) Priority() int { return r.priority }

func (r staticFailureRule) Evaluate(signal PodSignal, _ WorkloadContext) *DiagnosisDecision {
	if signal.FailureType != r.failureType {
		return nil
	}
	return &DiagnosisDecision{FailureType: r.failureType}
}

func defaultRuntimeRules() []RuntimeRule {
	// Priority order: infrastructure -> configuration -> dependency -> runtime/application.
	return []RuntimeRule{
		staticFailureRule{name: "imagepull", priority: 10, failureType: "ImagePullBackOff"},
		staticFailureRule{name: "failed-scheduling", priority: 11, failureType: "FailedScheduling"},
		staticFailureRule{name: "configmap-missing", priority: 20, failureType: "ConfigMapMissing"},
		staticFailureRule{name: "secret-missing", priority: 21, failureType: "SecretMissing"},
		staticFailureRule{name: "dns-failure", priority: 30, failureType: "DNSLookupFailed"},
		staticFailureRule{name: "network-timeout", priority: 31, failureType: "NetworkTimeout"},
		staticFailureRule{name: "rollout-failed", priority: 35, failureType: "DeploymentRolloutFailed"},
		staticFailureRule{name: "oom-killed", priority: 40, failureType: "OOMKilled"},
		staticFailureRule{name: "readiness-failed", priority: 41, failureType: "ReadinessProbeFailed"},
		staticFailureRule{name: "liveness-failed", priority: 42, failureType: "LivenessProbeFailed"},
		staticFailureRule{name: "crashloop", priority: 50, failureType: "CrashLoopBackOff"},
		staticFailureRule{name: "pod-pending", priority: 60, failureType: "PodPending"},
	}
}
