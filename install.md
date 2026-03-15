# Kuberoot Client Onboarding Guide

This guide is the end-to-end process to onboard a new client cluster.

## 1. One-time platform setup (owner side)

Do this once for your SaaS environment.

1. Railway backend variables must be set:
   - DATABASE_URL
   - INTERNAL_API_TOKEN
   - CORS_ORIGIN (recommended)
2. Vercel frontend variables must be set:
   - VITE_API_URL=https://kuberoot-production.up.railway.app
   - VITE_API_KEY=<a valid kr_live key>
   - VITE_CLUSTER_ID=<default cluster shown in UI, optional>
   - VITE_USE_PROXY=true (recommended)
3. Confirm backend health:
   curl -s https://kuberoot-production.up.railway.app/health

Expected output includes status ok.

## 2. Generate a client API key (owner side)

For every new client cluster, generate a unique key.

1. Set admin environment variables:
   export KUBEROOT_BACKEND_URL="https://kuberoot-production.up.railway.app"
   export INTERNAL_API_TOKEN="<your_internal_api_token>"
2. Generate key:
   ./scripts/generate_key.sh <organization_id> <cluster_id>

Example:
   ./scripts/generate_key.sh acme acme-prod-cluster

3. Save these values in your internal CRM/spreadsheet:
   - organization_id
   - cluster_id
   - api_key (kr_live_...)
   - date issued
   - contact owner

Important:
- Use a different cluster_id and key per cluster.
- Never share INTERNAL_API_TOKEN with clients.

## 3. What to send the client

Send only this install block (replace values):

Install Kuberoot Agent

export KUBEROOT_API_KEY="kr_live_xxxxx"
export KUBEROOT_CLUSTER_ID="my-cluster"

curl -sSL https://raw.githubusercontent.com/LikhithSM/KubeRoot/main/install.yaml \
 | envsubst \
 | kubectl apply -f -

Then verify:

kubectl get pods -n kuberoot
kubectl logs -n kuberoot deploy/kuberoot-agent

Open dashboard:
https://kube-root.vercel.app

## 4. What the client should expect

After install:
1. Namespace and agent resources are created in kuberoot namespace.
2. Agent pod becomes Running.
3. Agent polls cluster and sends failures to backend.
4. Dashboard starts showing failures for that cluster.

## 5. Owner verification checklist (after client installs)

Run these checks from your side.

1. Check backend history with that client key:
   curl -s -H "X-API-Key: <client_kr_live_key>" "https://kuberoot-production.up.railway.app/diagnose/history?cluster=<cluster_id>"
2. Confirm API returns JSON with items array.
3. Confirm UI proxy endpoint returns 200:
   curl -i -s "https://kube-root.vercel.app/api/history?cluster=<cluster_id>"
4. Confirm dashboard shows non-zero failures when failures exist.

## 6. Local end-to-end test flow (your rehearsal before first clients)

Use this exact sequence.

1. Generate test key:
   export KUBEROOT_BACKEND_URL="https://kuberoot-production.up.railway.app"
   export INTERNAL_API_TOKEN="<your_internal_api_token>"
   ./scripts/generate_key.sh demo demo-cluster
2. Install agent in your test cluster:
   export KUBEROOT_API_KEY="<generated_key>"
   export KUBEROOT_CLUSTER_ID="demo-cluster"
   curl -sSL https://raw.githubusercontent.com/LikhithSM/KubeRoot/main/install.yaml | envsubst | kubectl apply -f -
3. Verify rollout:
   kubectl rollout status deployment/kuberoot-agent -n kuberoot --timeout=90s
   kubectl get pods -n kuberoot
4. Check agent logs:
   kubectl logs -n kuberoot deploy/kuberoot-agent
5. Trigger a crash pod:
   kubectl apply -f - <<EOF
   apiVersion: v1
   kind: Pod
   metadata:
     name: crash-test
   spec:
     containers:
     - name: crash
       image: busybox
       command: ["sh","-c","exit 1"]
   EOF
6. Wait about 20 to 70 seconds, then verify history:
   curl -s -H "X-API-Key: <generated_key>" "https://kuberoot-production.up.railway.app/diagnose/history?cluster=demo-cluster"
7. Verify UI:
   https://kube-root.vercel.app

Cleanup:
- kubectl delete pod crash-test --ignore-not-found
- kubectl delete namespace kuberoot --ignore-not-found

## 7. Troubleshooting

1. Error: internal key generation not configured
   - INTERNAL_API_TOKEN is missing in Railway backend variables.
2. Error: invalid API key from /api/history
   - Vercel VITE_API_KEY is wrong or stale. Update and redeploy.
3. UI shows 0 failures but backend has data
   - Hard refresh browser (Cmd + Shift + R).
   - Confirm /api/history returns 200.
   - Confirm cluster query matches cluster_id used by agent.
4. ImagePullBackOff for kuberoot-agent
   - Confirm image exists on Docker Hub and pull policy is correct.
5. Agent is running but no data appears
   - Check agent logs for report errors.
   - Verify outbound internet access from cluster to Railway.

## 8. Notes on cluster_id vs kubectl context

cluster_id is an application identifier sent by the agent.
It does not need to match the kubectl context name.

Example:
- kubectl context: kind-debug-cluster
- KUBEROOT_CLUSTER_ID: demo-cluster
- UI and backend will show demo-cluster
