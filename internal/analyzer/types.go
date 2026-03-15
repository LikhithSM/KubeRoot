package analyzer

// PodSignal is the normalized runtime signal passed through the diagnosis pipeline.
type PodSignal struct {
	FailureType  string
	Namespace    string
	PodName      string
	Container    string
	Image        string
	RestartCount int32
	ExitCode     int32
	Message      string
	Events       []string
}

// WorkloadContext captures workload-level dependency context derived from the failing pod.
type WorkloadContext struct {
	Namespace          string
	Deployment         string
	DeploymentRevision string
	ReplicaStatus      string
	Image              string
	ContainerCommand   string
	ConfigMaps         []string
	Secrets            []string
	Services           []string
	EnvVariables       []string
	DependencyGraph    []string
}

// DiagnosisDecision is an optional override produced by validators/runtime rules.
type DiagnosisDecision struct {
	FailureType    string
	LikelyCause    string
	SuggestedFix   string
	Confidence     string
	ConfidenceNote string
}
