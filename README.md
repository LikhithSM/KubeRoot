# Install Kuberoot Agent

## SaaS key model (required)

- Every cluster must use a unique API key.
- Do not share one API key across multiple clusters.
- Use a readable cluster ID (for example, `acme-staging`), not a Kubernetes internal UID.

## Generate key (internal)

Use the internal endpoint to mint a key and optionally register a cluster.

### 1) Set INTERNAL_API_TOKEN on backend

Create a strong token:

```bash
python3 -c "import secrets; print(secrets.token_urlsafe(48))"
```

Set it in your backend environment:

- Local shell:

```bash
export INTERNAL_API_TOKEN='paste-strong-token'
export DATABASE_URL='postgresql://...'
go run ./cmd/server
```

- Docker Compose:
   - Set `backend.environment.INTERNAL_API_TOKEN` in `docker-compose.yml`.
   - Run `docker compose up -d --build`.

- Kubernetes:
   - Set `stringData.INTERNAL_API_TOKEN` in `deploy/k8s/backend-secret.yaml`.
   - Apply: `kubectl apply -f deploy/k8s/backend-secret.yaml -f deploy/k8s/backend-deployment.yaml`.

- Railway:
   - Add `INTERNAL_API_TOKEN` in Railway service Variables.
   - Redeploy service.

### 2) Generate cluster key

```bash
curl -X POST "$KUBEROOT_BACKEND_URL/internal/generate-key" \
   -H "Content-Type: application/json" \
   -H "X-Internal-Token: $INTERNAL_API_TOKEN" \
   -d '{
      "organizationId": "acme",
      "name": "acme-staging-agent",
      "clusterId": "acme-staging"
   }'
```

Response includes `apiKey` once. Store it securely.

Or use helper script:

```bash
chmod +x scripts/generate_key.sh
export KUBEROOT_BACKEND_URL='https://your-backend-url'
export INTERNAL_API_TOKEN='paste-strong-token'
./scripts/generate_key.sh acme acme-staging
```

1. Get an API key from the Kuberoot team.
2. Install (recommended):

```bash
export KUBEROOT_API_KEY=kr_live_xxx
export KUBEROOT_CLUSTER_ID=acme-staging

curl -sSL https://raw.githubusercontent.com/LikhithSM/KubeRoot/refs/heads/main/install.yaml \
   | envsubst \
   | kubectl apply -f -
```

If envsubst is not available:

```bash
curl -sSL https://raw.githubusercontent.com/LikhithSM/KubeRoot/refs/heads/main/install.yaml \
   | sed "s|\${KUBEROOT_API_KEY}|kr_live_xxx|g" \
   | sed "s|\${KUBEROOT_CLUSTER_ID}|acme-staging|g" \
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
