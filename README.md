# Install Kuberoot Agent

1. Get an API key from the Kuberoot team.
2. Install (recommended):

```bash
export KUBEROOT_API_KEY=kr_live_xxx
export KUBEROOT_CLUSTER_ID=$(kubectl get apiservices v1. -o jsonpath='{.metadata.uid}')

curl -sSL https://raw.githubusercontent.com/LikhithSM/KubeRoot/main/install.yaml \
   | envsubst \
   | kubectl apply -f -
```

If envsubst is not available:

```bash
curl -sSL https://raw.githubusercontent.com/LikhithSM/KubeRoot/main/install.yaml \
   | sed "s|\${KUBEROOT_API_KEY}|kr_live_xxx|g" \
   | sed "s|\${KUBEROOT_CLUSTER_ID}|$(kubectl get apiservices v1. -o jsonpath='{.metadata.uid}')|g" \
   | kubectl apply -f -
```

Verify:

```bash
kubectl get pods -n kuberoot
```

Troubleshooting:

- `404: Not Found` from the `curl` URL means the repository/file is not publicly reachable yet.
- `ImagePullBackOff` means `likhithsm/kuberoot-agent:latest` is not published to Docker Hub yet.

Uninstall:

```bash
kubectl delete namespace kuberoot
```
