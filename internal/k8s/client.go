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
)

type PodFailure struct {
	Namespace string
	Name      string
	Container string   // container name (if applicable)
	Types     []string // one or more of the above failure types
	Message   string   // optional: short message we can print now; events on Day 4
	Events    []string
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

	// 1) Container-level states (CrashLoopBackOff, ImagePullBackOff, OOMKilled)
	checkContainerStatuses := func(statuses []corev1.ContainerStatus) {
		for _, cs := range statuses {
			var types []string
			msg := ""

			// Waiting reasons
			if cs.State.Waiting != nil {
				reason := cs.State.Waiting.Reason
				switch reason {
				case "CrashLoopBackOff":
					types = append(types, string(FailureCrashLoopBackOff))
					msg = cs.State.Waiting.Message
				case "ImagePullBackOff":
					types = append(types, string(FailureImagePullBackOff))
					msg = cs.State.Waiting.Message
				}
			}

			// Terminated reasons
			if cs.State.Terminated != nil {
				reason := cs.State.Terminated.Reason
				if reason == "OOMKilled" {
					types = append(types, string(FailureOOMKilled))
					// message often empty here; keep it if present
					if cs.State.Terminated.Message != "" {
						msg = cs.State.Terminated.Message
					}
				}
			}

			if len(types) > 0 {
				results = append(results, PodFailure{
					Namespace: pod.Namespace,
					Name:      pod.Name,
					Container: cs.Name,
					Types:     types,
					Message:   msg,
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
				Namespace: pod.Namespace,
				Name:      pod.Name,
				Container: "", // N/A at pod level
				Types:     []string{string(FailureFailedScheduling)},
				Message:   cond.Message,
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

		recentEvents, eventsErr := getRecentPodEvents(ctx, cs, p.Namespace, p.Name, 3)
		if eventsErr != nil {
			return nil, fmt.Errorf("list events for pod %s/%s: %w", p.Namespace, p.Name, eventsErr)
		}

		for i := range failures {
			failures[i].Events = recentEvents
		}
		out = append(out, failures...)
	}
	return out, nil
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
