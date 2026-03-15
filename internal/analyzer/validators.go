package analyzer

import "strings"

// Validator runs dependency/context validation before runtime rules.
type Validator interface {
	Name() string
	Validate(signal PodSignal, ctx WorkloadContext) *DiagnosisDecision
}

type ConfigMapValidator struct{}

type SecretValidator struct{}

type ServiceDependencyValidator struct{}

func (v ConfigMapValidator) Name() string { return "configmap-validator" }

func (v ConfigMapValidator) Validate(signal PodSignal, ctx WorkloadContext) *DiagnosisDecision {
	if signal.FailureType == "ConfigMapMissing" {
		return &DiagnosisDecision{FailureType: "ConfigMapMissing"}
	}
	if len(ctx.ConfigMaps) == 0 {
		return nil
	}
	combined := strings.ToLower(strings.Join(append([]string{signal.Message}, signal.Events...), "\n"))
	if strings.Contains(combined, "configmap") && strings.Contains(combined, "not found") {
		return &DiagnosisDecision{FailureType: "ConfigMapMissing"}
	}
	return nil
}

func (v SecretValidator) Name() string { return "secret-validator" }

func (v SecretValidator) Validate(signal PodSignal, ctx WorkloadContext) *DiagnosisDecision {
	if signal.FailureType == "SecretMissing" {
		return &DiagnosisDecision{FailureType: "SecretMissing"}
	}
	if len(ctx.Secrets) == 0 {
		return nil
	}
	combined := strings.ToLower(strings.Join(append([]string{signal.Message}, signal.Events...), "\n"))
	if strings.Contains(combined, "secret") && strings.Contains(combined, "not found") {
		return &DiagnosisDecision{FailureType: "SecretMissing"}
	}
	return nil
}

func (v ServiceDependencyValidator) Name() string { return "service-dependency-validator" }

func (v ServiceDependencyValidator) Validate(signal PodSignal, _ WorkloadContext) *DiagnosisDecision {
	if signal.FailureType == "DNSLookupFailed" || signal.FailureType == "NetworkTimeout" {
		return &DiagnosisDecision{FailureType: signal.FailureType}
	}

	combined := strings.ToLower(strings.Join(append([]string{signal.Message}, signal.Events...), "\n"))
	if (strings.Contains(combined, "lookup") && strings.Contains(combined, "no such host")) || strings.Contains(combined, "temporary failure in name resolution") {
		return &DiagnosisDecision{FailureType: "DNSLookupFailed"}
	}
	if strings.Contains(combined, "i/o timeout") || strings.Contains(combined, "connection timed out") || strings.Contains(combined, "context deadline exceeded") {
		return &DiagnosisDecision{FailureType: "NetworkTimeout"}
	}

	return nil
}
