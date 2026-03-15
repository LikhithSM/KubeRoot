package k8s

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// --- Existing minimal model ---

type PodInfo struct {
	Namespace string
	Name      string
	Phase     string
}

// --- Day 3: Failure detection model ---

type FailureType string

const (
	FailureCrashLoopBackOff FailureType = "CrashLoopBackOff"
	FailureImagePullBackOff FailureType = "ImagePullBackOff"
	FailureOOMKilled        FailureType = "OOMKilled"
	FailureFailedScheduling FailureType = "FailedScheduling"
	FailureReadinessProbe   FailureType = "ReadinessProbeFailed"
	FailureLivenessProbe    FailureType = "LivenessProbeFailed"
	FailureConfigMapMissing FailureType = "ConfigMapMissing"
	FailureSecretMissing    FailureType = "SecretMissing"
	FailurePodPending       FailureType = "PodPending"
	FailureDNSLookup        FailureType = "DNSLookupFailed"
	FailureImageRegistryDNS FailureType = "ImageRegistryDNSFailure"
	FailureNetworkTimeout   FailureType = "NetworkTimeout"
	FailureRolloutFailed    FailureType = "DeploymentRolloutFailed"
)

type PodFailure struct {
	Namespace             string
	Name                  string
	Container             string // container name (if applicable)
	Image                 string
	Deployment            string
	DeploymentRevision    string
	ReplicaStatus         string
	Services              []string
	ConfigMaps            []string
	Secrets               []string
	EnvVariables          []string
	ContainerCommand      string
	Types                 []string // one or more of the above failure types
	Message               string   // optional: short message we can print now; events on Day 4
	Events                []string
	RestartCount          int32
	ContainerState        string
	WaitingReason         string
	TerminatedReason      string
	ExitCode              int32
	LastTerminationReason string
	LastExitCode          int32
	MemoryLimit           string
	CPURequest            string
	PodAgeSeconds         int64
	RecentRollout         bool
}

// --- Config helpers (unchanged) ---

func kubeconfigPath() (string, error) {
	if env := os.Getenv("KUBECONFIG"); env != "" {
		return env, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kube", "config"), nil
}

func LoadConfig() (*rest.Config, error) {
	if kc, err := kubeconfigPath(); err == nil {
		if _, statErr := os.Stat(kc); statErr == nil {
			if cfg, buildErr := clientcmd.BuildConfigFromFlags("", kc); buildErr == nil {
				return cfg, nil
			}
		}
	}
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	return nil, errors.New("unable to load kubeconfig or in-cluster config")
}

func NewClientset() (*kubernetes.Clientset, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("LoadConfig: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kubernetes.NewForConfig: %w", err)
	}
	return cs, nil
}

// --- Day 1–2: list pods (unchanged) ---

func ListPods(ctx context.Context, cs *kubernetes.Clientset) ([]PodInfo, error) {
	podList, err := cs.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	out := make([]PodInfo, 0, len(podList.Items))
	for _, p := range podList.Items {
		out = append(out, PodInfo{
			Namespace: p.Namespace,
			Name:      p.Name,
			Phase:     string(p.Status.Phase),
		})
	}
	return out, nil
}

// --- Day 3: failure detection ---

// DetectFailures scans pod statuses for well-known failure states.
func DetectFailures(pod corev1.Pod) []PodFailure {
	var results []PodFailure
	now := time.Now().UTC()
	podAgeSeconds := int64(0)
	if !pod.CreationTimestamp.IsZero() {
		podAgeSeconds = int64(now.Sub(pod.CreationTimestamp.Time).Seconds())
		if podAgeSeconds < 0 {
			podAgeSeconds = 0
		}
	}
	recentRollout := podAgeSeconds > 0 && podAgeSeconds <= 10*60

	// 1) Container-level states (CrashLoopBackOff, ImagePullBackOff, OOMKilled)
	checkContainerStatuses := func(statuses []corev1.ContainerStatus) {
		for _, cs := range statuses {
			var types []string
			msg := ""
			containerSpec, foundContainerSpec := findContainerSpec(pod.Spec.Containers, cs.Name)
			containerImage := cs.Image
			if foundContainerSpec && containerSpec.Image != "" {
				containerImage = containerSpec.Image
			}

			containerState := "Unknown"
			waitingReason := ""
			terminatedReason := ""
			exitCode := int32(0)
			lastTerminationReason := ""
			lastExitCode := int32(0)
			memoryLimit := ""
			cpuRequest := ""
			containerCommand := ""

			if foundContainerSpec {
				if q, ok := containerSpec.Resources.Limits[corev1.ResourceMemory]; ok {
					memoryLimit = q.String()
				}
				if q, ok := containerSpec.Resources.Requests[corev1.ResourceCPU]; ok {
					cpuRequest = q.String()
				}
				if len(containerSpec.Command) > 0 {
					containerCommand = strings.Join(containerSpec.Command, " ")
					if len(containerSpec.Args) > 0 {
						containerCommand = containerCommand + " " + strings.Join(containerSpec.Args, " ")
					}
				} else if len(containerSpec.Args) > 0 {
					containerCommand = strings.Join(containerSpec.Args, " ")
				}
			}

			// Waiting reasons
			if cs.State.Waiting != nil {
				containerState = "Waiting"
				reason := cs.State.Waiting.Reason
				waitingReason = reason
				switch reason {
				case "CrashLoopBackOff":
					types = appendType(types, string(FailureCrashLoopBackOff))
					msg = cs.State.Waiting.Message
				case "ImagePullBackOff", "ErrImagePull":
					types = appendType(types, string(FailureImagePullBackOff))
					msg = cs.State.Waiting.Message
				case "ContainerCreating":
					// Only flag pods stuck in ContainerCreating — likely a missing ConfigMap/Secret
					if podAgeSeconds > 30 {
						types = appendType(types, string(FailurePodPending))
						msg = cs.State.Waiting.Message
					}
				}
			}

			// Terminated reasons
			if cs.State.Terminated != nil {
				containerState = "Terminated"
				reason := cs.State.Terminated.Reason
				terminatedReason = reason
				exitCode = cs.State.Terminated.ExitCode
				if reason == "OOMKilled" {
					types = appendType(types, string(FailureOOMKilled))
					// message often empty here; keep it if present
					if cs.State.Terminated.Message != "" {
						msg = cs.State.Terminated.Message
					}
				}
			}

			if cs.State.Running != nil {
				containerState = "Running"
			}

			if cs.LastTerminationState.Terminated != nil {
				lastTerminationReason = cs.LastTerminationState.Terminated.Reason
				lastExitCode = cs.LastTerminationState.Terminated.ExitCode
				if lastTerminationReason == "OOMKilled" || lastExitCode == 137 {
					types = appendType(types, string(FailureOOMKilled))
				}
			}

			if len(types) > 0 {
				results = append(results, PodFailure{
					Namespace:             pod.Namespace,
					Name:                  pod.Name,
					Container:             cs.Name,
					Image:                 containerImage,
					Types:                 types,
					Message:               msg,
					RestartCount:          cs.RestartCount,
					ContainerState:        containerState,
					WaitingReason:         waitingReason,
					TerminatedReason:      terminatedReason,
					ExitCode:              exitCode,
					LastTerminationReason: lastTerminationReason,
					LastExitCode:          lastExitCode,
					MemoryLimit:           memoryLimit,
					CPURequest:            cpuRequest,
					ContainerCommand:      containerCommand,
					PodAgeSeconds:         podAgeSeconds,
					RecentRollout:         recentRollout,
				})
			}
		}
	}

	checkContainerStatuses(pod.Status.InitContainerStatuses)
	checkContainerStatuses(pod.Status.ContainerStatuses)

	// 2) Pod-level conditions (FailedScheduling)
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse {
			// Kubernetes typically sets Reason "Unschedulable" with explanatory Message.
			results = append(results, PodFailure{
				Namespace:     pod.Namespace,
				Name:          pod.Name,
				Container:     "", // N/A at pod level
				Types:         []string{string(FailureFailedScheduling)},
				Message:       cond.Message,
				PodAgeSeconds: podAgeSeconds,
				RecentRollout: recentRollout,
			})
		}
	}

	return results
}

// GetFailedPods returns only pods with detected failures plus details per container.
func GetFailedPods(ctx context.Context, cs *kubernetes.Clientset) ([]PodFailure, error) {
	podList, err := cs.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods for failures: %w", err)
	}
	var out []PodFailure
	for _, p := range podList.Items {
		failures := DetectFailures(p)
		if len(failures) == 0 {
			continue
		}

		recentEvents, eventsErr := getRecentPodEvents(ctx, cs, p.Namespace, p.Name, 8)
		if eventsErr != nil {
			return nil, fmt.Errorf("list events for pod %s/%s: %w", p.Namespace, p.Name, eventsErr)
		}

		for i := range failures {
			failures[i].Events = recentEvents
			enrichFailureWithEventSignals(&failures[i])
			enrichFailureWithWorkloadContext(ctx, cs, p, &failures[i])
		}
		out = append(out, failures...)
	}
	return out, nil
}

func enrichFailureWithWorkloadContext(ctx context.Context, cs *kubernetes.Clientset, pod corev1.Pod, failure *PodFailure) {
	if failure == nil {
		return
	}

	failure.Deployment, failure.DeploymentRevision, failure.ReplicaStatus = resolveDeploymentStatus(ctx, cs, pod)
	failure.Services = listMatchingServices(ctx, cs, pod)
	failure.ConfigMaps, failure.Secrets, failure.EnvVariables = collectPodConfigRefs(pod, failure.Container)
}

func resolveDeploymentStatus(ctx context.Context, cs *kubernetes.Clientset, pod corev1.Pod) (string, string, string) {
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "Deployment" {
			dep, err := cs.AppsV1().Deployments(pod.Namespace).Get(ctx, owner.Name, metav1.GetOptions{})
			if err != nil {
				return owner.Name, "", ""
			}
			desired := int32(1)
			if dep.Spec.Replicas != nil {
				desired = *dep.Spec.Replicas
			}
			revision := dep.Annotations["deployment.kubernetes.io/revision"]
			return owner.Name, revision, fmt.Sprintf("%d/%d", dep.Status.ReadyReplicas, desired)
		}
		if owner.Kind == "ReplicaSet" {
			rs, err := cs.AppsV1().ReplicaSets(pod.Namespace).Get(ctx, owner.Name, metav1.GetOptions{})
			if err != nil {
				return "", "", ""
			}
			for _, rsOwner := range rs.OwnerReferences {
				if rsOwner.Kind == "Deployment" {
					dep, depErr := cs.AppsV1().Deployments(pod.Namespace).Get(ctx, rsOwner.Name, metav1.GetOptions{})
					if depErr != nil {
						return rsOwner.Name, "", ""
					}
					desired := int32(1)
					if dep.Spec.Replicas != nil {
						desired = *dep.Spec.Replicas
					}
					revision := dep.Annotations["deployment.kubernetes.io/revision"]
					return rsOwner.Name, revision, fmt.Sprintf("%d/%d", dep.Status.ReadyReplicas, desired)
				}
			}
		}
	}

	return "", "", ""
}

func listMatchingServices(ctx context.Context, cs *kubernetes.Clientset, pod corev1.Pod) []string {
	if len(pod.Labels) == 0 {
		return nil
	}

	svcs, err := cs.CoreV1().Services(pod.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}

	matches := make([]string, 0, 4)
	for _, svc := range svcs.Items {
		if len(svc.Spec.Selector) == 0 {
			continue
		}
		ok := true
		for k, v := range svc.Spec.Selector {
			if podVal, exists := pod.Labels[k]; !exists || podVal != v {
				ok = false
				break
			}
		}
		if ok {
			matches = append(matches, svc.Name)
		}
	}

	sort.Strings(matches)
	return matches
}

func collectPodConfigRefs(pod corev1.Pod, targetContainer string) ([]string, []string, []string) {
	configMaps := make(map[string]struct{})
	secrets := make(map[string]struct{})
	envVars := make(map[string]struct{})

	for _, vol := range pod.Spec.Volumes {
		if vol.ConfigMap != nil && strings.TrimSpace(vol.ConfigMap.Name) != "" {
			configMaps[vol.ConfigMap.Name] = struct{}{}
		}
		if vol.Secret != nil && strings.TrimSpace(vol.Secret.SecretName) != "" {
			secrets[vol.Secret.SecretName] = struct{}{}
		}
	}

	containers := pod.Spec.Containers
	for _, c := range containers {
		if targetContainer != "" && c.Name != targetContainer {
			continue
		}
		for _, env := range c.Env {
			if strings.TrimSpace(env.Name) != "" {
				envVars[env.Name] = struct{}{}
			}
			if env.ValueFrom != nil {
				if env.ValueFrom.ConfigMapKeyRef != nil && strings.TrimSpace(env.ValueFrom.ConfigMapKeyRef.Name) != "" {
					configMaps[env.ValueFrom.ConfigMapKeyRef.Name] = struct{}{}
				}
				if env.ValueFrom.SecretKeyRef != nil && strings.TrimSpace(env.ValueFrom.SecretKeyRef.Name) != "" {
					secrets[env.ValueFrom.SecretKeyRef.Name] = struct{}{}
				}
			}
		}
		for _, envFrom := range c.EnvFrom {
			if envFrom.ConfigMapRef != nil && strings.TrimSpace(envFrom.ConfigMapRef.Name) != "" {
				configMaps[envFrom.ConfigMapRef.Name] = struct{}{}
			}
			if envFrom.SecretRef != nil && strings.TrimSpace(envFrom.SecretRef.Name) != "" {
				secrets[envFrom.SecretRef.Name] = struct{}{}
			}
		}
	}

	if targetContainer != "" && len(envVars) == 0 {
		for _, c := range containers {
			for _, env := range c.Env {
				if strings.TrimSpace(env.Name) != "" {
					envVars[env.Name] = struct{}{}
				}
			}
		}
	}

	return mapKeys(configMaps), mapKeys(secrets), mapKeys(envVars)
}

func mapKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func appendType(types []string, t string) []string {
	for _, existing := range types {
		if existing == t {
			return types
		}
	}
	return append(types, t)
}

func findContainerSpec(containers []corev1.Container, name string) (corev1.Container, bool) {
	for _, c := range containers {
		if c.Name == name {
			return c, true
		}
	}
	return corev1.Container{}, false
}

func enrichFailureWithEventSignals(failure *PodFailure) {
	configMapHit := false
	secretHit := false

	for _, event := range failure.Events {
		lower := strings.ToLower(event)

		if strings.Contains(lower, "readiness probe failed") {
			failure.Types = appendType(failure.Types, string(FailureReadinessProbe))
		}
		if strings.Contains(lower, "liveness probe failed") {
			failure.Types = appendType(failure.Types, string(FailureLivenessProbe))
		}

		// ConfigMap mount failure: "MountVolume.SetUp failed ... configmap "name" not found"
		isMountFail := strings.Contains(lower, "mountvolume") || strings.Contains(lower, "mount failed")
		if isMountFail && strings.Contains(lower, "configmap") && (strings.Contains(lower, "not found") || strings.Contains(lower, "failed")) {
			configMapHit = true
		}
		// Secret mount failure
		if isMountFail && strings.Contains(lower, "secret") && (strings.Contains(lower, "not found") || strings.Contains(lower, "failed")) {
			secretHit = true
		}

		if strings.Contains(lower, "lookup") && strings.Contains(lower, "no such host") {
			if isImagePullDNSFailure(lower) {
				failure.Types = appendType(failure.Types, string(FailureImageRegistryDNS))
			} else {
				failure.Types = appendType(failure.Types, string(FailureDNSLookup))
			}
		}
		if strings.Contains(lower, "i/o timeout") || strings.Contains(lower, "connection timed out") || strings.Contains(lower, "context deadline exceeded") {
			failure.Types = appendType(failure.Types, string(FailureNetworkTimeout))
		}
		if strings.Contains(lower, "progress deadline exceeded") || strings.Contains(lower, "timed out progressing") {
			failure.Types = appendType(failure.Types, string(FailureRolloutFailed))
		}
	}

	if configMapHit {
		failure.Types = removeType(failure.Types, string(FailurePodPending))
		failure.Types = appendType(failure.Types, string(FailureConfigMapMissing))
	}
	if secretHit {
		failure.Types = removeType(failure.Types, string(FailurePodPending))
		failure.Types = appendType(failure.Types, string(FailureSecretMissing))
	}
}

func removeType(types []string, t string) []string {
	out := make([]string, 0, len(types))
	for _, existing := range types {
		if existing != t {
			out = append(out, existing)
		}
	}
	return out
}

func isImagePullDNSFailure(eventLower string) bool {
	if strings.Contains(eventLower, "pull") && strings.Contains(eventLower, "image") {
		return true
	}
	if strings.Contains(eventLower, "errimagepull") || strings.Contains(eventLower, "imagepullbackoff") {
		return true
	}
	if strings.Contains(eventLower, "registry") {
		return true
	}
	return false
}

func getRecentPodEvents(ctx context.Context, cs *kubernetes.Clientset, namespace, podName string, limit int) ([]string, error) {
	selector := fmt.Sprintf("involvedObject.kind=Pod,involvedObject.name=%s", podName)
	eventList, err := cs.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{FieldSelector: selector})
	if err != nil {
		return nil, err
	}

	type eventEntry struct {
		text string
		ts   time.Time
	}
	entries := make([]eventEntry, 0, len(eventList.Items))
	seen := make(map[string]struct{})

	for _, event := range eventList.Items {
		reason := strings.TrimSpace(event.Reason)
		message := strings.TrimSpace(event.Message)
		if reason == "" && message == "" {
			continue
		}

		text := reason
		if reason != "" && message != "" {
			text = reason + ": " + message
		} else if reason == "" {
			text = message
		}

		if _, ok := seen[text]; ok {
			continue
		}
		seen[text] = struct{}{}

		entries = append(entries, eventEntry{
			text: text,
			ts:   eventTimestamp(event),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ts.After(entries[j].ts)
	})

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.text)
	}
	return out, nil
}

func eventTimestamp(event corev1.Event) time.Time {
	if !event.EventTime.IsZero() {
		return event.EventTime.Time
	}
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	if event.Series != nil && !event.Series.LastObservedTime.IsZero() {
		return event.Series.LastObservedTime.Time
	}
	if !event.FirstTimestamp.IsZero() {
		return event.FirstTimestamp.Time
	}
	if !event.CreationTimestamp.IsZero() {
		return event.CreationTimestamp.Time
	}
	return time.Time{}
}
