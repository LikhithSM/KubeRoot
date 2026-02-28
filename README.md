# Install Kuberoot Agent

1. Get an API key from the Kuberoot team.
2. Install (recommended):

```bash
export KUBEROOT_API_KEY=kr_live_xxx
export KUBEROOT_CLUSTER_ID=acme-staging

curl -sSL https://raw.githubusercontent.com/LikhithSM/kuberoot/main/install.yaml \
   | envsubst \
   | kubectl apply -f -
```

If envsubst is not available:

```bash
curl -sSL https://raw.githubusercontent.com/LikhithSM/kuberoot/main/install.yaml \
   | sed "s|\${KUBEROOT_API_KEY}|kr_live_xxx|g" \
   | sed "s|\${KUBEROOT_CLUSTER_ID}|acme-staging|g" \
   | kubectl apply -f -
```

Verify:

```bash
kubectl get pods -n kuberoot
```

Uninstall:

```bash
kubectl delete namespace kuberoot
```
