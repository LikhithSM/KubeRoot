package analyzer

import (
	"strings"

	"kuberoot/internal/k8s"
)

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

func buildPodSignal(failureType string, failure k8s.PodFailure) PodSignal {
	exitCode := failure.ExitCode
	if exitCode == 0 {
		exitCode = failure.LastExitCode
	}
	return PodSignal{
		FailureType:  failureType,
		Namespace:    failure.Namespace,
		PodName:      failure.Name,
		Container:    failure.Container,
		Image:        failure.Image,
		RestartCount: failure.RestartCount,
		ExitCode:     exitCode,
		Message:      failure.Message,
		Events:       failure.Events,
	}
}

func buildWorkloadContext(failure k8s.PodFailure) WorkloadContext {
	ctx := WorkloadContext{
		Namespace:          failure.Namespace,
		Deployment:         failure.Deployment,
		DeploymentRevision: failure.DeploymentRevision,
		ReplicaStatus:      failure.ReplicaStatus,
		Image:              failure.Image,
		ContainerCommand:   failure.ContainerCommand,
		ConfigMaps:         append([]string{}, failure.ConfigMaps...),
		Secrets:            append([]string{}, failure.Secrets...),
		Services:           append([]string{}, failure.Services...),
		EnvVariables:       append([]string{}, failure.EnvVariables...),
	}
	ctx.DependencyGraph = buildDependencyGraph(ctx)
	return ctx
}

func buildDependencyGraph(ctx WorkloadContext) []string {
	nodes := make([]string, 0, 16)

	if strings.TrimSpace(ctx.Deployment) != "" {
		nodes = append(nodes, "Deployment "+ctx.Deployment)
	}
	for _, cm := range ctx.ConfigMaps {
		nodes = append(nodes, "ConfigMap "+cm)
	}
	for _, secret := range ctx.Secrets {
		nodes = append(nodes, "Secret "+secret)
	}
	for _, svc := range ctx.Services {
		nodes = append(nodes, "Service "+svc)
	}
	if strings.TrimSpace(ctx.Image) != "" {
		nodes = append(nodes, "Image "+ctx.Image)
	}

	return uniqueStrings(nodes)
}
