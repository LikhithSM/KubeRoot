package analyzer

import (
	"time"

	"kuberoot/internal/k8s"
)

// DiagnosisDecision is an optional override produced by validators/rules.
type DiagnosisDecision struct {
	FailureType    string
	LikelyCause    string
	SuggestedFix   string
	Confidence     string
	ConfidenceNote string
}

type RuntimeRule interface {
	Name() string
	Priority() int
	Evaluate(signal PodSignal, ctx WorkloadContext) *DiagnosisDecision
}

type DiagnosisEngine struct {
	ruleMap    map[string]Rule
	validators []Validator
	rules      []RuntimeRule
}

func NewDiagnosisEngine(baseRules []Rule) *DiagnosisEngine {
	ruleMap := make(map[string]Rule, len(baseRules))
	for _, rule := range baseRules {
		ruleMap[rule.FailureType] = rule
	}

	return &DiagnosisEngine{
		ruleMap: ruleMap,
		validators: []Validator{
			ConfigMapValidator{},
			SecretValidator{},
			ServiceDependencyValidator{},
		},
		rules: defaultRuntimeRules(),
	}
}

func (e *DiagnosisEngine) Diagnose(orgID, clusterID string, failure k8s.PodFailure, failureType string) (Diagnosis, bool) {
	signal := buildPodSignal(failureType, failure)
	ctx := buildWorkloadContext(failure)

	for _, validator := range e.validators {
		if decision := validator.Validate(signal, ctx); decision != nil {
			return e.composeDiagnosis(orgID, clusterID, failure, decision, failureType)
		}
	}

	for _, rule := range e.rules {
		if decision := rule.Evaluate(signal, ctx); decision != nil {
			return e.composeDiagnosis(orgID, clusterID, failure, decision, failureType)
		}
	}

	return e.composeDiagnosis(orgID, clusterID, failure, nil, failureType)
}

func (e *DiagnosisEngine) composeDiagnosis(orgID, clusterID string, failure k8s.PodFailure, decision *DiagnosisDecision, fallbackType string) (Diagnosis, bool) {
	effectiveType := fallbackType
	if decision != nil && decision.FailureType != "" {
		effectiveType = decision.FailureType
	}

	rule, exists := e.ruleMap[effectiveType]
	if !exists {
		return Diagnosis{}, false
	}

	evidence := buildEvidence(effectiveType, failure)
	ctx := buildContextSignals(failure)
	ctx = append(ctx, buildDependencyGraph(buildWorkloadContext(failure))...)
	likelyCause := deriveLikelyCause(rule.LikelyCause, effectiveType, failure, evidence)
	fixSuggestions := buildFixSuggestions(effectiveType, failure, evidence)
	suggestedFix := deriveSuggestedFix(rule.SuggestedFix, effectiveType, failure, evidence, fixSuggestions)
	quickCommands := buildQuickCommands(effectiveType, failure, evidence)
	confidence, confidenceNote := enrichConfidence(rule.Confidence, effectiveType, failure, evidence)
	severity := computeSeverity(confidence, effectiveType, failure)

	if decision != nil {
		if decision.LikelyCause != "" {
			likelyCause = decision.LikelyCause
		}
		if decision.SuggestedFix != "" {
			suggestedFix = decision.SuggestedFix
		}
		if decision.Confidence != "" {
			confidence = decision.Confidence
		}
		if decision.ConfidenceNote != "" {
			confidenceNote = decision.ConfidenceNote
		}
	}

	return Diagnosis{
		OrganizationID: orgID,
		ClusterID:      clusterID,
		PodName:        failure.Name,
		Namespace:      failure.Namespace,
		Container:      failure.Container,
		Image:          failure.Image,
		RestartCount:   failure.RestartCount,
		FailureType:    effectiveType,
		Category:       categorizeFailure(effectiveType),
		Severity:       severity,
		LikelyCause:    likelyCause,
		SuggestedFix:   suggestedFix,
		Confidence:     confidence,
		ConfidenceNote: confidenceNote,
		Evidence:       evidence,
		FixSuggestions: fixSuggestions,
		QuickCommands:  quickCommands,
		Context:        uniqueStrings(ctx),
		Events:         failure.Events,
		Timestamp:      time.Now().UTC(),
	}, true
}
