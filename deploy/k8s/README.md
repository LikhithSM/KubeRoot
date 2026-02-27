# Kuberoot in-cluster deployment (k8s)

This folder contains raw manifests for a minimal in-cluster deployment:
- Postgres
- Backend
- Agent

## Prereqs
- `kubectl` configured for your cluster
- Images built and loaded into the cluster runtime

## 1) Build images

From repo root:

```
docker build -t kuberoot-backend:local -f Dockerfile .
docker build -t kuberoot-agent:local -f Dockerfile.agent .
```

If using kind:

```
kind load docker-image kuberoot-backend:local
kind load docker-image kuberoot-agent:local
```

## 2) Configure secrets

Update these files before apply:
- postgres-secret.yaml (password)
- backend-secret.yaml (DATABASE_URL)
- agent-secret.yaml (KUBEROOT_API_KEY)
- agent-configmap.yaml (KUBEROOT_CLUSTER_ID)

## 3) Apply manifests

```
kubectl apply -f deploy/k8s
```

## 4) Verify

```
kubectl -n kuberoot get pods
kubectl -n kuberoot get svc
kubectl -n kuberoot logs deploy/kuberoot-backend
kubectl -n kuberoot logs deploy/kuberoot-agent
```

## 5) Check health

```
kubectl -n kuberoot port-forward svc/kuberoot-backend 8080:8080
curl http://localhost:8080/health
```
