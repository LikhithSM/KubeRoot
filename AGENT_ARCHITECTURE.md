# KUBEROOT AGENT ARCHITECTURE
## Cluster-Side Push Model for SaaS Viability

---

## 🎯 THE PROBLEM WITH CURRENT ARCHITECTURE

**Current (Broken for SaaS)**:
```
Your Laptop/Server             →  Kubernetes Cluster
                kubeconfig
            (requires admin)
     ↓
 Kuberoot Backend
 • Holds kubeconfig
 • Pulls pod data
 • Runs analysis
```

**Why This Fails**:
- ❌ Users must give you kubeconfig
- ❌ Kuberoot needs cluster-admin access
- ❌ No security boundary
- ❌ Enterprise says "absolutely not"
- ❌ Not a SaaS product, it's a liability

---

## ✅ THE SOLUTION: PUSH MODEL WITH AGENT

```
Kubernetes Cluster (Customer's)
    ↓
[Kuberoot Agent Pod]
  • Detects failures
  • Sends HTTPS payload
  • Needs: viewer + pod read access
                ↓
        Kuberoot SaaS Backend (Your Server)
          • Receives payload
          • Runs analyzer
          • Stores diagnosis
          • Returns insights
```

**Why This Works**:
- ✅ Agent uses minimal RBAC (read-only, no admin)
- ✅ Backend never sees kubeconfig
- ✅ Clear security boundary
- ✅ Enterprise trust model
- ✅ This is a real SaaS product
- ✅ Scalable to hundreds of clusters

---

## 📐 AGENT ARCHITECTURE

### Pod Specification

```yaml
# kuberoot-agent.yaml (customer applies once)
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kuberoot-agent
  namespace: kuberoot-system

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kuberoot-agent
rules:
# Read pods and their status
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
# Read events
- apiGroups: [""]
  resources: ["events"]
  verbs: ["get", "list"]
# No other permissions needed

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kuberoot-agent
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kuberoot-agent
subjects:
- kind: ServiceAccount
  name: kuberoot-agent
  namespace: kuberoot-system

---
apiVersion: v1
kind: Secret
metadata:
  name: kuberoot-api-key
  namespace: kuberoot-system
type: Opaque
stringData:
  api-key: "kr_YOUR_API_KEY_HERE"

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kuberoot-agent
  namespace: kuberoot-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kuberoot-agent
  template:
    metadata:
      labels:
        app: kuberoot-agent
    spec:
      serviceAccountName: kuberoot-agent
      containers:
      - name: agent
        image: kuberoot:agent-v1
        imagePullPolicy: IfNotPresent
        env:
        - name: KUBEROOT_BACKEND_URL
          value: "https://api.kuberoot.io"
        - name: KUBEROOT_API_KEY
          valueFrom:
            secretKeyRef:
              name: kuberoot-api-key
              key: api-key
        - name: KUBEROOT_CLUSTER_ID
          value: "prod-us-east-1"  # Customer-friendly identifier
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 30
```

### Agent Detection Loop

```go
// cmd/agent/main.go - Runs inside cluster as Pod

func main() {
    // Load in-cluster config (automatic when in Pod)
    config, _ := rest.InClusterConfig()
    clientset, _ := kubernetes.NewForConfig(config)
    
    // Every 30 seconds (configurable)
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        // 1. Detect current failures
        failures := k8s.GetFailedPods(ctx, clientset)
        
        // 2. Build payload with cluster context
        payload := AgentPayload{
            ClusterID: os.Getenv("KUBEROOT_CLUSTER_ID"),
            Timestamp: time.Now().UTC(),
            Failures: failures,
        }
        
        // 3. Send to backend via HTTPS
        sendToBackend(payload)
    }
}
```

---

## 📡 AGENT ↔ BACKEND PROTOCOL

### Push Endpoint: `POST /api/v1/agent/report`

**Agent sends** (every 30 seconds):
```json
{
  "clusterID": "prod-us-east-1",
  "timestamp": "2026-02-25T10:30:00Z",
  "failures": [
    {
      "namespace": "default",
      "podName": "app-pod-123",
      "container": "app",
      "failureType": "CrashLoopBackOff",
      "message": "Application exited with code 1",
      "events": [
        "Created: Container created",
        "Started: Container started",
        "BackOff: Back-off restarting failed container"
      ]
    }
  ]
}
```

**Backend authenticates with X-API-Key header**
```
POST /api/v1/agent/report
X-API-Key: kr_abc123...
Content-Type: application/json
{
  "clusterID": "prod-us-east-1",
  ...
}
```

**Backend responses**:
```
200 OK - Accepted
401 Unauthorized - Invalid API key
429 Too Many Requests - Rate limited
500 Internal Error - Retry later
```

### Backend Processing

```go
// internal/api/agent_handler.go

func (h *Handler) AgentReport(w http.ResponseWriter, r *http.Request) {
    // 1. Auth middleware already validated orgID
    orgID := auth.GetOrganizationID(r.Context())
    
    // 2. Parse payload
    var payload AgentPayload
    json.NewDecoder(r.Body).Decode(&payload)
    
    // 3. Diagnose failures (same analyzer logic)
    diagnoses := analyzer.DiagnoseFailures(
        orgID,
        payload.ClusterID,
        payload.Failures,
    )
    
    // 4. Store diagnosis
    store.SaveDiagnoses(ctx, orgID, payload.ClusterID, diagnoses)
    
    // 5. Return immediate response
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "accepted",
        "id": "report-uuid",
    })
}
```

---

## 🔄 COMPARISON: Old vs New

### Old Model (Current)
```
Backend → [kubeconfig] → Queries Cluster
• Backend has admin access
• Periodic polling (slow)
• Single cluster only
• Not SaaS-ready
```

### New Model (Agent)
```
Agent Pod → [HTTPS] → Backend
• Agent has read-only access
• Real-time push (fast)
• Multi-cluster capable
• Enterprise-ready
• Scalable
```

---

## 📋 IMPLEMENTATION ROADMAP

### Phase 1: Agent Bootstrap (This Week)
1. Create `cmd/agent/main.go`
2. Use in-cluster config
3. Poll failures every 30 seconds
4. Send to backend via HTTP
5. Test locally with Kind

### Phase 2: Backend Agent Endpoint (2 Days)
1. Add `POST /api/v1/agent/report` handler
2. Parse AgentPayload
3. Run analyzer
4. Store diagnosis
5. Return 200 OK

### Phase 3: Helm Chart (3 Days)
1. Create Helm chart for easy deployment
2. Parametrize API key, backend URL, cluster ID
3. Single `helm install kuberoot/agent` command

### Phase 4: Docker Compose Stack (2 Days)
1. Backend Dockerfile
2. Agent Dockerfile
3. docker-compose with Postgres + Backend + Prometheus
4. `docker compose up` for local testing

---

## 🚀 NEXT IMMEDIATE STEPS

### Step 1: Build Agent (Recommended First)
```
cmd/agent/main.go
├── Load in-cluster config
├── Create k8s clientset
├── Poll failures every 30s
├── Send via HTTPS
└── Auth with API key
```

**Why first**: This unblocks everything else
- UI can display data from agent
- Beta users need agent to use platform
- Agent + backend = complete product

### Step 2: Add Agent Endpoint
```
internal/api/
└── agent_handler.go
    ├── POST /api/v1/agent/report
    ├── Parse payload
    ├── Run analyzer
    └── Store diagnosis
```

### Step 3: Build React Dashboard
```
dashboard/
├── App.tsx
├── pages/
│   ├── Failures.tsx (table)
│   └── FailureDetail.tsx (detail view)
├── components/
│   ├── FailureTable.tsx
│   └── EventList.tsx
└── api/
    └── client.ts
```

### Step 4: Dockerize Everything
```
docker-compose.yml
├── postgres
├── backend (Go)
├── agent (Go)
└── dashboard (React)
```

---

## 💾 MINIMAL IMPLEMENTATION OPTIONS

### Option A: Agent First (Recommended)
**Time**: 1 day
**Code changes**:
1. `cmd/agent/main.go` (~200 lines)
2. `internal/api/agent_handler.go` (~100 lines)
3. New endpoint registration

**Why**: Unblocks all future work

---

### Option B: UI First
**Time**: 2-3 days
**Code changes**:
1. Create React app
2. Build Failures table
3. Build Detail view
4. Wire to `/diagnose/history` endpoint

**Why**: Users want UI, not curl

---

### Option C: Docker Stack First
**Time**: 1 day
**Code changes**:
1. Create Dockerfile for backend
2. Create Dockerfile for agent
3. Create docker-compose.yml
4. Add health check endpoints

**Why**: Frictionless onboarding

---

## ✅ DECISION FRAMEWORK

**If you want SaaS-ready today**: Agent first
**If you want visual feedback**: UI first
**If you want frictionless demo**: Docker first

**Correct business order**:
1. Agent (enables everything)
2. UI (makes it usable)
3. Docker (enables demos)
4. 3 beta users (validates product)

---

## 📊 POST-AGENT DATA FLOW

```
1. Customer applies agent YAML
2. Agent Pod starts in their cluster
3. Agent detects failures every 30s
4. Agent sends HTTPS payload to backend
5. Backend analyzes + stores
6. UI shows results in real-time
7. Customer sees diagnosis without giving you cluster-admin
```

---

## 🔐 SECURITY MODEL

**What Agent Can Do**:
- ✅ List pods
- ✅ Read pod status
- ✅ Get events
- ✅ Send data out

**What Agent Cannot Do**:
- ❌ Modify pods
- ❌ Delete anything
- ❌ Access secrets
- ❌ Access cluster-admin

**API Key Security**:
- Stored in Secret (Kubernetes-managed)
- Hashed on backend (SHA-256)
- Only visible during creation
- Can be rotated, revoked instantly

---

## 🎯 SUCCESS CRITERIA

### Agent Works When:
- [ ] Pod starts in Kind cluster
- [ ] No errors in logs
- [ ] Pushes data every 30s
- [ ] Backend receives and stores
- [ ] UI shows failures in real-time
- [ ] 3 beta users can deploy with ease

### SaaS Works When:
- [ ] Each customer has unique API key
- [ ] Data is org-scoped (multi-tenant)
- [ ] No manual steps (no kubeconfig sharing)
- [ ] Takes <5 minutes to onboard cluster
- [ ] First user reports "saved me 20 minutes"

---

## 📝 IMMEDIATE ACTION

**Start with this question**:
Which do you want to build first?
1. **Agent** (cmd/agent/main.go) → Backend integration
2. **UI** (React dashboard) → Visual feedback
3. **Docker stack** → Deployment story

I recommend **Agent** because:
- Smallest scope (1 day)
- Unblocks everything else
- Makes product SaaS-ready
- Enables real user onboarding

Ready to build it?
