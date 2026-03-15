# Debug Lab Examples (Failure + Healthy Pods)

This file creates a lab namespace with:
- 1 healthy workload (`good-pod`)
- failure workloads for every diagnosis path currently implemented

## Apply

```bash
kubectl apply -f deploy/k8s/debug-lab-examples.yaml
```

## Verify Pods

```bash
kubectl get pods -n debug-lab
```

## Example -> Expected Failure Type in KubeRoot

- `good-pod` -> healthy baseline (should not appear as active issue)
- `crashloop-demo` -> `CrashLoopBackOff`
- `imagepull-demo` -> `ImagePullBackOff`
- `oom-demo` -> `OOMKilled`
- `failedschedule-demo` -> `FailedScheduling`
- `readiness-fail-demo` -> `ReadinessProbeFailed`
- `liveness-fail-demo` -> `LivenessProbeFailed`
- `configmap-missing-demo` -> `ConfigMapMissing`
- `secret-missing-demo` -> `SecretMissing`
- `dnslookup-demo` -> `DNSLookupFailed` (probe DNS host lookup failure)
- `network-timeout-demo` -> `NetworkTimeout` (probe dial timeout)
- `rollout-timeout-demo` -> `DeploymentRolloutFailed` / rollout-stall signal path

## Where Exact Fix Appears in UI

1. Open Active Issues in dashboard.
2. Click any failing pod row.
3. In the detail panel:
   - `Recommended Fix` shows the human-readable resolution.
   - `Exact Fix (Patch Snippet)` shows a direct YAML patch when available.
   - `Exact Fix Commands` shows copy/paste kubectl commands.

## Cleanup

```bash
kubectl delete -f deploy/k8s/debug-lab-examples.yaml
```
