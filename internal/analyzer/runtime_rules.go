package analyzer

import "strings"

type runtimeRuleFunc struct {
	evaluate func(signal PodSignal, ctx WorkloadContext) *DiagnosisDecision
}

func (r runtimeRuleFunc) Evaluate(signal PodSignal, ctx WorkloadContext) *DiagnosisDecision {
	if r.evaluate == nil {
		return nil
	}
	return r.evaluate(signal, ctx)
}

func detectCrashLoop(signal PodSignal, _ WorkloadContext) *DiagnosisDecision {
	if signal.FailureType != "CrashLoopBackOff" {
		return nil
	}
	return &DiagnosisDecision{FailureType: "CrashLoopBackOff"}
}

func detectImagePull(signal PodSignal, _ WorkloadContext) *DiagnosisDecision {
	if signal.FailureType != "ImagePullBackOff" {
		return nil
	}
	return &DiagnosisDecision{FailureType: "ImagePullBackOff"}
}

func detectOOM(signal PodSignal, _ WorkloadContext) *DiagnosisDecision {
	if signal.FailureType == "OOMKilled" || signal.ExitCode == 137 {
		return &DiagnosisDecision{FailureType: "OOMKilled"}
	}
	combined := strings.ToLower(strings.Join(append([]string{signal.Message}, signal.Events...), "\n"))
	if strings.Contains(combined, "oomkilled") || strings.Contains(combined, "out of memory") {
		return &DiagnosisDecision{FailureType: "OOMKilled"}
	}
	return nil
}

func detectProbeFailure(signal PodSignal, _ WorkloadContext) *DiagnosisDecision {
	if signal.FailureType == "ReadinessProbeFailed" || signal.FailureType == "LivenessProbeFailed" {
		return &DiagnosisDecision{FailureType: signal.FailureType}
	}
	return nil
}

func detectScheduling(signal PodSignal, _ WorkloadContext) *DiagnosisDecision {
	if signal.FailureType == "FailedScheduling" {
		return &DiagnosisDecision{FailureType: "FailedScheduling"}
	}
	return nil
}

func detectPending(signal PodSignal, _ WorkloadContext) *DiagnosisDecision {
	if signal.FailureType == "PodPending" {
		return &DiagnosisDecision{FailureType: "PodPending"}
	}
	return nil
}

func detectRollout(signal PodSignal, _ WorkloadContext) *DiagnosisDecision {
	if signal.FailureType == "DeploymentRolloutFailed" {
		return &DiagnosisDecision{FailureType: "DeploymentRolloutFailed"}
	}
	return nil
}

func defaultRuntimeRules() []RuntimeRule {
	// Ordered with lightweight infrastructure/runtime checks first.
	return []RuntimeRule{
		runtimeRuleFunc{evaluate: detectImagePull},
		runtimeRuleFunc{evaluate: detectScheduling},
		runtimeRuleFunc{evaluate: detectRollout},
		runtimeRuleFunc{evaluate: detectOOM},
		runtimeRuleFunc{evaluate: detectProbeFailure},
		runtimeRuleFunc{evaluate: detectCrashLoop},
		runtimeRuleFunc{evaluate: detectPending},
	}
}
