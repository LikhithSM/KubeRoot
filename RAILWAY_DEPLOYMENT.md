# 🚀 Railway Deployment Guide (Backend Only)

## Overview
This guide deploys **ONLY the backend** to Railway.
- ✅ Backend service (Go binary)
- ✅ PostgreSQL database (Railway addon)
- ❌ NOT the React UI (deploy separately later to Vercel)
- ❌ NOT the agent (runs in customer clusters)
- ❌ NOT Kubernetes files (for customer install, not SaaS)

---

## Prerequisites
1. GitHub account with forked kuberoot repo
2. Railway.app account (free tier)
3. Access to Railway dashboard

---

## Step 1: Create Railway Project

```bash
# Visit: https://railway.app/dashboard
# Click: "New Project"
# Select: "Deploy from GitHub repo"
# Authorize Railway to access your GitHub account
# Select: kuberoot repository
```

✅ Railway detects Go project + Dockerfile

---

## Step 2: Add PostgreSQL Database

```bash
# In Railway Dashboard (your project):
# Click: "+ Add Services"
# Select: "PostgreSQL"
# Railway auto-creates Postgres 15 instance
# Auto-generates DATABASE_URL env var
```

Example auto-generated DATABASE_URL:
```
postgresql://postgres:xxxxx@containers.railway.app:5432/railway
```

**What Railway does:**
- ✅ Creates Postgres database
- ✅ Sets DATABASE_URL environment variable
- ✅ Exposes to backend service automatically

---

## Step 3: Configure Backend Service

```bash
# In Railway Dashboard:
# Click on the Go service (auto-detected from Dockerfile)
# Settings → Variables
```

**Required Environment Variables:**

| Name | Value | Source |
|------|-------|--------|
| `DATABASE_URL` | `${{ Postgres.DATABASE_URL }}` | Use Railway template variable from Postgres service |
| `KUBEROOT_CLUSTER_ID` | `saas-backend` | Set manually (or leave default) |
| `PORT` | DO NOT SET | Railway injects this automatically; do NOT override |
| `CORS_ORIGIN` | (optional) | Leave blank for now (allows all) |

**No other config needed.**

---

## PORT handling in your Go server

Railway injects the `PORT` environment variable; do not set it manually in the dashboard. In your Go server use the following pattern to read it with a safe fallback for local development:

```go
port := os.Getenv("PORT")
if port == "" {
  port = "8080" // local dev fallback
}

log.Printf("Starting on :%s", port)
http.ListenAndServe(":"+port, handler)
```

This ensures the app will work both on Railway and locally.


## Step 4: Deploy

```bash
# Push code to GitHub:
git add -A
git commit -m "Fix Dockerfile: CGO_ENABLED=0 for Railway ARM64"
git push origin main

# Railway auto-detects push and triggers build
# Watch: Dashboard → Build logs

# Expected output:
# ✓ Build started
# ✓ FROM golang:1.25
# ✓ RUN go mod download
# ✓ RUN CGO_ENABLED=0 go build -o kuberoot ./cmd/server
# ✓ FROM debian:bookworm-slim
# ✓ COPY --from=builder /build/kuberoot /usr/local/bin/kuberoot
# ✓ Build successful → Deploying
```

---

## Step 5: Verify Deployment

Once Railway says "Deploy successful", test the backend:

### 5a. Health Check (No Auth Required)

```bash
# Get your Railway URL from dashboard
# Example: https://kuberoot-api-xxxx.railway.app

curl https://kuberoot-api-xxxx.railway.app/health

# Expected response:
{
  "status": "ok",
  "version": "1.0.0",
  "ready": true
}
```

✅ If you get `ready: true` → Backend is healthy

---

### 5b. View Logs

```bash
# In Railway Dashboard:
# Click service → Logs tab

# Should see:
# 🚀 Kuberoot backend starting on :8080 (SaaS mode, database-backed)
# [GET] /health | 2ms
```

---

### 5c. Test Agent Report (Simulate)

First, create an API key:

```bash
# Option A: Use keygen (if available):
# go run ./cmd/keygen/main.go
# Output example: kr_live_abc123xyz

# Option B: Generate a test key:
export API_KEY="kr_live_test_$(uuidgen | tr -d '-')"
echo "Test API Key: $API_KEY"
```

Then POST a test report:

```bash
curl -X POST https://kuberoot-api-xxxx.railway.app/api/v1/agent/report \
  -H "X-API-Key: kr_live_abc123xyz" \
  -H "Content-Type: application/json" \
  -d '{
    "clusterId": "test-cluster",
    "timestamp": "2026-02-27T00:00:00Z",
    "failures": [
      {
        "podName": "test-pod",
        "namespace": "default",
        "status": "CrashLoopBackOff"
      }
    ]
  }'

# Expected response:
{
  "status": "accepted",
  "id": "test-cluster",
  "message": "processed diagnoses"
}
```

✅ If you get `status: accepted` → Agent protocol works

---

## Step 6: Check Data Persisted (Pro)

To verify data is actually in Postgres:

```bash
# In Railway Dashboard:
# Click PostgreSQL service → Connect tab
# Copy connection string

psql $DATABASE_URL

# Then in psql:
SELECT * FROM diagnoses LIMIT 1;
SELECT COUNT(*) FROM diagnoses;
SELECT * FROM clusters;
```

✅ If you see rows → Database persistence works

---

## Troubleshooting

### Build Error: "CGO assembly error"
**Cause:** Old Dockerfile with `CGO_ENABLED=1 GOARCH=arm64`
**Fix:** Push updated Dockerfile with `CGO_ENABLED=0`
```bash
git add Dockerfile && git commit -m "Fix: CGO_ENABLED=0" && git push
```

### Deployment Error: "failed to solve: EOF"
**Cause:** Network timeout or Railway service issue
**Fix:** Retry deployment
```bash
# Push empty commit to trigger new build:
git commit --allow-empty -m "Retry build"
git push origin main
```

### Health Check Returns Error
**Cause:** DATABASE_URL not set in Railway
**Check:**
```bash
# In Railway Dashboard → Variables
# DATABASE_URL should be auto-set by Postgres service
# If missing, manually add it
```

### Agent Report Returns 401 Unauthorized
**Cause:** Invalid or missing X-API-Key
**Fix:** Ensure API key is valid in database
```bash
# This is expected for test keys
# In production, use keygen to create real keys
```

---

## What NOT to Deploy Here (Yet)

❌ **React UI** → Deploy to Vercel/Netlify separately  
❌ **Agent** → Runs in customer clusters via Kubernetes  
❌ **Kubernetes manifests** → For customer installation  
❌ **Docker Compose** → For local dev only  

**Why separate?**
- Backend needs high availability (Railway)
- UI can be static/CDN (Vercel)
- Agent is customer-managed (Kubernetes)
- Each component scales independently

---

## Next Steps (After Backend Stable)

Once you confirm:
- ✅ `/health` returns `ready: true`
- ✅ Agent POST succeeds
- ✅ Data persists in Postgres
- ✅ Logs are clean (no errors for 30+ min)

**Then:**
1. Deploy React UI to Vercel
2. Point UI to your Railway backend URL
3. Test end-to-end
4. Beta user onboarding

---

## Quick Reference

| What | Where |
|------|-------|
| **Dashboard** | https://railway.app/dashboard |
| **Build Logs** | Dashboard → Service → Build |
| **Runtime Logs** | Dashboard → Service → Logs |
| **Environment Variables** | Dashboard → Service → Variables |
| **Database Connection** | Dashboard → PostgreSQL → Connect |
| **Health Endpoint** | `GET https://your-url/health` |
| **Agent Report** | `POST https://your-url/api/v1/agent/report` |

---

## Success Criteria ✅

Your backend is **production-ready** when:

- [x] Health endpoint returns `{"status":"ok","ready":true}`
- [x] Logs show "🚀 Kuberoot backend starting on :8080"
- [x] No error messages in logs for 5+ minutes
- [x] Agent POST returns `{"status":"accepted"}`
- [x] PostgreSQL has data in `diagnoses` table
- [x] CPU/RAM stable (no spikes)

Once all checks pass → You have a live SaaS backend! 🎉

---

**Current Status:** Backend ready for Railway  
**Next:** Push to Railway and confirm health check  
**Time:** ~5 minutes
