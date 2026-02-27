# KUBEROOT - Complete Architecture Guide

## 🎯 What Is This?

**Kuberoot** is a **Kubernetes Incident Intelligence Platform** that automatically:
1. Detects pod failures in your cluster
2. Correlates them with Kubernetes events
3. Diagnoses the root cause
4. Suggests fixes with confidence scores
5. Stores history for pattern analysis

Think of it as **"Auto-Triage for Kubernetes"** - instead of manually running `kubectl describe pod` and searching through events, Kuberoot does it for you and tells you exactly what's wrong.

---

## 🏗️ High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         HTTP REQUEST                            │
│           curl http://localhost:8080/diagnose                   │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                     AUTHENTICATION LAYER                        │
│  ┌──────────────────┐              ┌──────────────────┐         │
│  │  LocalMode       │              │  ProductionMode  │         │
│  │  Middleware      │              │  APIKey          │         │
│  │  (dev)           │              │  Middleware      │         │
│  │                  │              │  (multi-tenant)  │         │
│  │ Injects:         │              │ Validates:       │         │
│  │ "local-org"      │              │ X-API-Key header │         │
│  └──────────────────┘              └──────────────────┘         │
└────────────────────────────┬────────────────────────────────────┘
                             │ Request Context has orgID
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                        API HANDLERS                             │
│  ┌──────────────────────────────────────────────────────┐       │
│  │  /diagnose        - Live failure detection           │       │
│  │  /diagnose/history - Historical query with filters   │       │
│  └──────────────────────────────────────────────────────┘       │
└────────────────────────────┬────────────────────────────────────┘
                             │
         ┌───────────────────┴──────────────────┐
         ▼                                      ▼
┌──────────────────────┐            ┌──────────────────────┐
│  KUBERNETES CLIENT   │            │  STORAGE LAYER       │
│                      │            │                      │
│ • List all pods      │            │ • NoopStore (local)  │
│ • Check statuses     │            │ • PostgresStore      │
│ • Fetch events       │            │   (production)       │
│ • Detect failures    │            │                      │
└──────────┬───────────┘            └──────────┬───────────┘
           │                                   │
           ▼                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                      ANALYZER ENGINE                            │
│  ┌──────────────────────────────────────────────────────┐       │
│  │  Rule Matching:                                      │       │
│  │  • CrashLoopBackOff → Check logs & config           │       │
│  │  • ImagePullBackOff → Verify image/registry         │       │
│  │  • OOMKilled        → Increase memory limits         │       │
│  │                                                      │       │
│  │  Confidence Enrichment:                              │       │
│  │  • Base confidence from rule                         │       │
│  │  • +1 if events contain matching keywords            │       │
│  │  • -1 if no events found                             │       │
│  └──────────────────────────────────────────────────────┘       │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                          ▼
                  JSON Response to Client
```

---

## 📂 Component Breakdown

### 1. **cmd/server/main.go** - Application Entry Point

**Purpose**: Bootstrap the server and wire dependencies together.

**Flow**:
```go
1. Load Kubernetes config (kubeconfig or in-cluster)
2. Create Kubernetes clientset
3. Check if DATABASE_URL is set:
   - YES → Initialize PostgresStore + API key auth
   - NO  → Use NoopStore + local mode auth
4. Create HTTP handler with routes
5. Apply authentication middleware
6. Start HTTP server on :8080
```

**Key Decision**: 
- **Environment-driven config**: No hardcoded values, everything via env vars
- **Conditional middleware**: Different auth based on DATABASE_URL presence

---

### 2. **internal/k8s/client.go** - Kubernetes Interaction

**Purpose**: Talk to Kubernetes API to detect pod failures and fetch events.

#### Core Functions:

**`NewClientset()`**
- Loads kubeconfig from `~/.kube/config` or `KUBECONFIG` env var
- Falls back to in-cluster config (for when Kuberoot runs inside K8s)
- Requires OIDC plugin for certain cloud providers

**`GetFailedPods(ctx, clientset)`**
- Lists ALL pods across ALL namespaces
- For each pod, calls `DetectFailures()` to check status
- If failures found, fetches recent events via `getRecentPodEvents()`
- Returns `[]PodFailure` with enriched event data

**`DetectFailures(pod)`**
- Inspects `pod.Status.ContainerStatuses` for:
  - **Waiting state** → CrashLoopBackOff, ImagePullBackOff
  - **Terminated state** → OOMKilled
- Inspects `pod.Status.Conditions` for:
  - **Unschedulable** → FailedScheduling
- Returns all detected failure types per pod

**`getRecentPodEvents(ctx, cs, namespace, podName, limit)`**
- Queries Kubernetes Events API with field selector
- Filters for events related to specific pod
- Deduplicates events (same reason+message)
- Sorts by timestamp (newest first)
- Returns last N events (default: 3)

**Why 3 events?**
- Balance between context and noise
- Typically enough to show failure progression
- Keeps JSON response size manageable

---

### 3. **internal/analyzer/rules.go** - Diagnosis Engine

**Purpose**: Match detected failures to known patterns and suggest fixes.

#### Data Model:

```go
type Diagnosis struct {
    OrganizationID string    // Multi-tenant isolation
    ClusterID      string    // Which cluster this came from
    PodName        string    
    Namespace      string
    FailureType    string    // CrashLoopBackOff, etc.
    LikelyCause    string    // Human-readable root cause
    SuggestedFix   string    // Actionable remediation
    Confidence     string    // low/medium/high
    Events         []string  // Recent K8s events
    Timestamp      time.Time
}
```

#### The V1 Rule Set:

**Rule 1: CrashLoopBackOff**
```
Cause: Application crash or configuration error
Fix:   Check application logs; verify env vars; validate ConfigMap/Secret mounts
Base Confidence: medium
```

**Rule 2: OOMKilled**
```
Cause: Container exceeded memory limit
Fix:   Increase memory limit; inspect usage; investigate memory leaks
Base Confidence: high
```

**Rule 3: ImagePullBackOff**
```
Cause: Image pull failed (name/registry/credentials)
Fix:   Verify image name; check registry access; validate pull secret
Base Confidence: high
```

#### Confidence Enrichment Algorithm:

```go
1. Start with rule's base confidence (low=1, medium=2, high=3)
2. Check events for matching keywords:
   - CrashLoopBackOff: "back-off restarting", "crashloopbackoff", "failed"
   - OOMKilled:        "oomkilled", "out of memory", "killing"
   - ImagePullBackOff: "failed to pull", "pull access denied", "manifest unknown"
3. If keywords found → +1 to score
4. If NO events at all → -1 to score (less confident without evidence)
5. Cap final score at 3
6. Convert back to string (1=low, 2=medium, 3=high)
```

**Why this matters**: 
- Events provide **evidence-based confidence**
- Pure rule matching (no events) is less reliable
- Event keywords confirm the diagnosis

---

### 4. **internal/api/handler.go** - HTTP Request Handlers

#### **Handler 1: `/diagnose` (Live Diagnosis)**

**Request Flow**:
```
1. Extract orgID from request context (set by auth middleware)
2. Call k8s.GetFailedPods() → Get current cluster failures
3. Call analyzer.DiagnoseFailures() → Match to rules + enrich with events
4. Call store.SaveDiagnoses() → Persist to database (if enabled)
5. Return JSON response with all diagnoses
```

**Response Structure**:
```json
{
  "cluster": "kind-debug-cluster",
  "failures": [
    {
      "organizationId": "local-org",
      "clusterId": "kind-debug-cluster",
      "podName": "crashloop-demo",
      "namespace": "debug-lab",
      "failureType": "CrashLoopBackOff",
      "likelyCause": "Application crash or configuration error",
      "suggestedFix": "Check application logs...",
      "confidence": "medium",
      "events": [
        "Created: Container created",
        "Started: Container started",
        "Pulled: Successfully pulled image..."
      ],
      "timestamp": "2026-02-24T07:30:29.609463Z"
    }
  ]
}
```

#### **Handler 2: `/diagnose/history` (Historical Query)**

**Query Parameters**:
- `cluster` (string, optional): which cluster’s history to fetch. defaults to the server’s own `KUBEROOT_CLUSTER_ID` when unset, but SaaS clients should provide the target cluster ID (e.g. `?cluster=test-staging-eks`).
- `limit` (int): Max results (default 50, max 200)
- `failureType` (string): Filter by CrashLoopBackOff|ImagePullBackOff|OOMKilled
- `namespace` (string): Filter by Kubernetes namespace
- `since` (RFC3339): Start time window
- `until` (RFC3339): End time window

**Request Flow**:
```
1. Extract orgID from context
2. Parse query parameters into DiagnosisHistoryFilter
3. Validate time window (since must be before until)
4. Call store.ListDiagnoses() with filters
5. Return paginated results
```

**SQL Query (PostgreSQL mode)**:
```sql
SELECT * FROM diagnoses
WHERE organization_id = $1        -- Multi-tenant isolation
  AND cluster_id = $2
  AND failure_type = $3           -- Optional filter
  AND namespace = $4              -- Optional filter
  AND created_at >= $5            -- Optional time window
  AND created_at <= $6
ORDER BY created_at DESC
LIMIT $7
```

---

### 5. **internal/auth/middleware.go** - Authentication

#### **LocalModeMiddleware()** (Development)

```go
// No API key required - auto-injects "local-org"
func LocalModeMiddleware() {
    return func(next http.Handler) {
        return func(w, r *http.Request) {
            ctx := context.WithValue(r.Context(), "organizationID", "local-org")
            next.ServeHTTP(w, r.WithContext(ctx))
        }
    }
}
```

**When Used**: DATABASE_URL not set (local development)

#### **APIKeyMiddleware(validator)** (Production)

```go
1. Extract X-API-Key header from request
2. Hash the key with SHA-256
3. Call validator.ValidateAPIKey(keyHash)
   → Queries: SELECT organization_id FROM api_keys WHERE key_hash = $1 AND active = true
   → Updates: last_used_at = NOW()
4. If valid → Inject organizationID into request context
5. If invalid → Return 401 Unauthorized
```

**Security Properties**:
- Keys stored as SHA-256 hashes (irreversible)
- Database lookup on every request (can revoke instantly)
- `last_used_at` tracking for audit/billing
- Active flag for soft deletion

**When Used**: DATABASE_URL is set (production mode)

---

### 6. **internal/store/** - Persistence Layer

#### **Interface Design** (`store.go`):

```go
type DiagnosisStore interface {
    SaveDiagnoses(ctx, orgID, clusterID, diagnoses) error
    ListDiagnoses(ctx, orgID, clusterID, filter) ([]Diagnosis, error)
    ValidateAPIKey(ctx, keyHash) (organizationID, error)
}
```

**Why an interface?**
- Local mode doesn't need a database (NoopStore)
- Production mode uses PostgreSQL (PostgresStore)
- Easy to swap implementations (Redis, S3, etc.)
- Testable without real database

#### **NoopStore** (Local Development):

```go
func (NoopStore) SaveDiagnoses(...) error {
    return nil  // Silently discard
}

func (NoopStore) ListDiagnoses(...) ([]Diagnosis, error) {
    return []Diagnosis{}, nil  // Empty history
}

func (NoopStore) ValidateAPIKey(...) (string, error) {
    return "local-org", nil  // Always allow with fake org
}
```

#### **PostgresStore** (Production):

**Schema**:
```sql
-- Organizations are implicitly defined via clusters
CREATE TABLE clusters (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
);

-- API keys for authentication
CREATE TABLE api_keys (
    id BIGSERIAL PRIMARY KEY,
    organization_id TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,   -- SHA-256 hash of the key
    name TEXT NOT NULL,               -- Human-friendly label
    active BOOLEAN DEFAULT true,      -- Soft delete flag
    created_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ          -- For audit/billing
);

-- Diagnosis history
CREATE TABLE diagnoses (
    id BIGSERIAL PRIMARY KEY,
    organization_id TEXT NOT NULL,    -- Multi-tenant isolation
    cluster_id TEXT NOT NULL REFERENCES clusters(id),
    pod_name TEXT NOT NULL,
    namespace TEXT NOT NULL,
    failure_type TEXT NOT NULL,
    likely_cause TEXT NOT NULL,
    suggested_fix TEXT NOT NULL,
    confidence TEXT NOT NULL,
    events JSONB NOT NULL,            -- Array of event strings
    created_at TIMESTAMPTZ
);
```

**Key Indexes**:
- `idx_api_keys_key_hash` → Fast auth lookups
- `idx_diagnoses_cluster_created_at` → Fast history queries

**Transaction Safety**:
```go
SaveDiagnoses() uses a transaction:
1. BEGIN
2. UPSERT into clusters (create if new, update timestamp if exists)
3. INSERT each diagnosis
4. COMMIT (or ROLLBACK on error)
```

---

### 7. **cmd/keygen/main.go** - API Key Generation

**Purpose**: Create new API keys for onboarding customers/teams.

**Flow**:
```bash
$ DATABASE_URL='postgres://...' go run ./cmd/keygen --org acme-corp --name prod-key

1. Generate 32 random bytes
2. Encode as "kr_<hex>" (kr = kuberoot prefix)
3. Hash with SHA-256
4. Output SQL INSERT statement
5. Admin manually runs SQL to activate key
```

**Output Example**:
```
Generated API Key:
========================================
Key: kr_5787dcca5881e3cbf4395cc306b50fea9ca0358fe27092bc170655d9869485f4
Organization: acme-corp
Name: prod-key

To activate, run this SQL:
========================================
INSERT INTO api_keys (organization_id, key_hash, name, active, created_at)
VALUES ('acme-corp', '30181258b0b4b9d106fcebec3fe05191e299d8fa875beeacb7286e1451defccc', 'prod-key', true, NOW());
```

**Security Note**: Key is shown ONCE. Database stores only the hash.

---

## 🔄 Complete Request Lifecycle

### **Scenario: DevOps Engineer Debugging a CrashLoop**

**Step 1: Engineer makes request**
```bash
curl http://localhost:8080/diagnose
```

**Step 2: HTTP Request arrives at server**
- Hits `main.go` HTTP server on `:8080`
- Enters middleware layer

**Step 3: Authentication Middleware**
```
IF local mode:
  → LocalModeMiddleware injects orgID = "local-org"
  → Proceed to handler

IF production mode:
  → APIKeyMiddleware extracts X-API-Key header
  → Hashes key with SHA-256
  → Queries database: SELECT organization_id FROM api_keys WHERE key_hash = ?
  → If valid: Inject real orgID into context
  → If invalid: Return 401 Unauthorized
```

**Step 4: Handler receives request**
```go
handler.Diagnose(w, r):
  1. orgID := auth.GetOrganizationID(r.Context())  // "local-org" or "acme-corp"
  2. failures := k8s.GetFailedPods(ctx, clientset)
```

**Step 5: Kubernetes Client scans cluster**
```go
k8s.GetFailedPods():
  1. List ALL pods across ALL namespaces
  2. For each pod:
     a. Check container statuses:
        - pod.Status.ContainerStatuses[].State.Waiting.Reason
          → "CrashLoopBackOff" detected!
     b. Fetch recent events:
        - Query Events API for pod
        - Deduplicate & sort by timestamp
        - Return last 3 events
  3. Return []PodFailure with events attached
```

**Events found**:
```
- "Created: Container created"
- "Started: Container started" 
- "BackOff: Back-off restarting failed container"
```

**Step 6: Analyzer diagnoses the failure**
```go
analyzer.DiagnoseFailures(orgID, clusterID, failures):
  1. Match "CrashLoopBackOff" to Rule 1:
     - Likely Cause: "Application crash or configuration error"
     - Suggested Fix: "Check application logs; verify env vars..."
     - Base Confidence: "medium"
  
  2. Enrich confidence with events:
     - Event contains "back-off restarting" → +1 evidence score
     - Final confidence: medium (2) + evidence (1) = high (3)
  
  3. Return Diagnosis struct with all details
```

**Step 7: Store diagnosis in database**
```go
store.SaveDiagnoses(ctx, orgID, clusterID, diagnoses):
  IF NoopStore:
    → Do nothing (local mode)
  
  IF PostgresStore:
    → BEGIN transaction
    → UPSERT cluster record
    → INSERT diagnosis with events as JSONB
    → COMMIT
```

**Step 8: Return JSON response**
```json
{
  "cluster": "kind-debug-cluster",
  "failures": [
    {
      "organizationId": "local-org",
      "podName": "crashloop-demo",
      "failureType": "CrashLoopBackOff",
      "likelyCause": "Application crash or configuration error",
      "suggestedFix": "Check application logs; verify environment variables...",
      "confidence": "high",
      "events": [
        "Created: Container created",
        "Started: Container started",
        "BackOff: Back-off restarting failed container"
      ],
      "timestamp": "2026-02-24T07:30:29.609463Z"
    }
  ]
}
```

**Step 9: Engineer sees the diagnosis**
- Knows exactly what's wrong (CrashLoopBackOff)
- Has suggested fix (check logs, verify config)
- Has evidence (K8s events show "back-off restarting")
- Can act immediately instead of debugging for 30 minutes

---

## 🔀 Mode Comparison

### **Local Mode** (DATABASE_URL not set)

**Use Case**: Local development, quick testing, proof-of-concept

| Feature | Behavior |
|---------|----------|
| **Authentication** | None - auto-injects "local-org" |
| **Storage** | NoopStore - diagnoses discarded |
| **History** | Always empty |
| **Multi-tenancy** | N/A |
| **Speed** | Fastest (no DB roundtrip) |

**How to run**:
```bash
KUBEROOT_CLUSTER_ID='my-cluster' go run ./cmd/server
# No DATABASE_URL = local mode
```

---

### **Production Mode** (DATABASE_URL set)

**Use Case**: Multi-tenant SaaS, customer deployments, long-term analytics

| Feature | Behavior |
|---------|----------|
| **Authentication** | Required - validates X-API-Key header |
| **Storage** | PostgreSQL - full persistence |
| **History** | Queryable with filters |
| **Multi-tenancy** | Isolated by organization_id |
| **Speed** | Slower (DB writes on every diagnose) |

**How to run**:
```bash
DATABASE_URL='postgres://user:pass@localhost:5432/kuberoot?sslmode=disable' \
KUBEROOT_CLUSTER_ID='prod-cluster' \
go run ./cmd/server
```

**Generate API key**:
```bash
DATABASE_URL='...' go run ./cmd/keygen --org customer-123 --name production
```

**Use API**:
```bash
curl -H "X-API-Key: kr_5787dc..." http://localhost:8080/diagnose
```

---

## 🧠 Technical Decisions & Rationale

### **1. Why only 3 rules in V1?**
- **Pareto Principle**: These 3 cover ~80% of production pod failures
- **Confidence matters**: Better to diagnose 3 things well than 20 things poorly
- **Fast iteration**: Easier to validate hypothesis with small rule set
- **Extensible**: Clean architecture makes adding rules trivial

### **2. Why store events as JSONB instead of separate table?**
- **Query simplicity**: Events are always fetched WITH diagnosis
- **Immutability**: Events don't change after diagnosis
- **Size**: 3 events per diagnosis = ~500 bytes (acceptable overhead)
- **Future-proof**: JSONB allows rich queries if needed later

### **3. Why middleware instead of handler-level auth?**
- **Single responsibility**: Handlers focus on business logic
- **Reusability**: Same middleware for all routes
- **Context injection**: orgID available to ALL handlers automatically
- **Security**: Can't accidentally forget to check auth

### **4. Why NoopStore instead of in-memory store?**
- **Clarity**: LocalMode is for development, not production
- **Simplicity**: No need to manage in-memory state
- **Memory safety**: Can't leak memory in long-running dev sessions
- **Upgrade path**: Forces users to use PostgreSQL for real history

### **5. Why SHA-256 for API keys instead of bcrypt?**
- **Performance**: SHA-256 is faster (important for auth on every request)
- **Collision resistance**: Good enough for 256-bit keys
- **Deterministic**: Same key always hashes to same value (for lookups)
- **Not for passwords**: We're hashing random keys, not user passwords

### **6. Why client-go instead of REST API calls?**
- **Type safety**: Compile-time checks for Kubernetes API changes
- **OIDC support**: Works with EKS, AKS, GKE out of the box
- **Retry logic**: Built-in exponential backoff for transient failures
- **Community standard**: What everyone uses for K8s tooling

---

## 📊 Data Flow Diagrams

### **Live Diagnosis Flow**

```
┌─────────┐
│  curl   │
│/diagnose│
└────┬────┘
     │
     ▼
┌────────────────┐
│ Auth Middleware│──────┐
│ (inject orgID) │      │
└────┬───────────┘      │
     │                  │ Request Context
     ▼                  │ now has orgID
┌──────────────┐        │
│  Handler     │◄───────┘
│ .Diagnose()  │
└──────┬───────┘
       │
       ├──────────────────────────┐
       ▼                          ▼
┌───────────────┐         ┌──────────────┐
│ k8s.          │         │ analyzer.    │
│ GetFailedPods │────────▶│ Diagnose     │
│               │ failures│ Failures     │
│ • List pods   │         │              │
│ • Check status│         │ • Match rules│
│ • Fetch events│         │ • Enrich     │
└───────────────┘         └──────┬───────┘
                                 │ diagnoses
                                 ▼
                          ┌──────────────┐
                          │ store.Save   │
                          │ Diagnoses    │
                          │              │
                          │ NoopStore or │
                          │ PostgresStore│
                          └──────┬───────┘
                                 │
                                 ▼
                          ┌──────────────┐
                          │ JSON Response│
                          └──────────────┘
```

### **History Query Flow**

```
┌─────────┐
│  curl   │
│/diagnose│
│/history │
│?since=  │
│&type=   │
└────┬────┘
     │
     ▼
┌────────────────┐
│ Auth Middleware│──────┐
│ (get orgID)    │      │
└────┬───────────┘      │
     │                  │
     ▼                  ▼
┌──────────────────────────┐
│  Handler                 │
│ .DiagnoseHistory()       │
│                          │
│ 1. Parse query params    │
│ 2. Build filter struct   │
│ 3. Validate time window  │
└──────┬───────────────────┘
       │
       ▼
┌──────────────────────────┐
│ store.ListDiagnoses()    │
│                          │
│ IF NoopStore:            │
│   return []              │
│                          │
│ IF PostgresStore:        │
│   SELECT * FROM diagnoses│
│   WHERE org_id = $1      │
│     AND cluster_id = $2  │
│     AND type = $3        │
│     AND created >= $4    │
│   ORDER BY created DESC  │
│   LIMIT $5               │
└──────┬───────────────────┘
       │
       ▼
┌──────────────┐
│ JSON Response│
│ {            │
│  cluster,    │
│  count,      │
│  items[]     │
│ }            │
└──────────────┘
```

---

## 🚀 Deployment Scenarios

### **Scenario 1: Single Developer - Mac Laptop**

```bash
# Prerequisites
brew install go kubernetes-cli kind
kind create cluster --name debug-cluster

# Run Kuberoot
cd kuberoot
KUBEROOT_CLUSTER_ID='kind-debug-cluster' go run ./cmd/server

# Test
curl http://localhost:8080/diagnose | jq .
```

**Mode**: Local (no DATABASE_URL)  
**Auth**: None  
**Storage**: NoopStore (no history)

---

### **Scenario 2: Small Team - Shared Kubernetes**

```bash
# Start PostgreSQL
docker run -d --name kuberoot-db \
  -e POSTGRES_PASSWORD=secret \
  -e POSTGRES_DB=kuberoot \
  -p 5432:5432 \
  postgres:16

# Generate API key for team
DATABASE_URL='postgres://postgres:secret@localhost:5432/kuberoot?sslmode=disable' \
go run ./cmd/keygen --org myteam --name main-key

# Run server
DATABASE_URL='postgres://postgres:secret@localhost:5432/kuberoot?sslmode=disable' \
KUBEROOT_CLUSTER_ID='staging-cluster' \
go run ./cmd/server

# Team members use API key
export APIKEY=kr_5787dc...
curl -H "X-API-Key: $APIKEY" http://localhost:8080/diagnose
curl -H "X-API-Key: $APIKEY" 'http://localhost:8080/diagnose/history?limit=10'
```

**Mode**: Production (DATABASE_URL set)  
**Auth**: API key required  
**Storage**: PostgreSQL (full history)  
**Multi-tenancy**: Single org ("myteam")

---

### **Scenario 3: SaaS Multi-Tenant**

```yaml
# Kubernetes Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kuberoot
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: kuberoot
        image: kuberoot:v1
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: postgres-creds
              key: url
        - name: KUBEROOT_CLUSTER_ID
          value: "prod-us-east-1"
        ports:
        - containerPort: 8080
```

**For each customer**:
```bash
# Generate unique API key
DATABASE_URL='...' go run ./cmd/keygen --org customer-acme --name prod
DATABASE_URL='...' go run ./cmd/keygen --org customer-xyz --name prod

# Customer A gets: kr_abc123...
# Customer B gets: kr_def456...
```

**Data isolation**:
```sql
-- Customer A can only see their data
SELECT * FROM diagnoses WHERE organization_id = 'customer-acme';

-- Customer B can only see their data
SELECT * FROM diagnoses WHERE organization_id = 'customer-xyz';
```

**Mode**: Production  
**Auth**: API key per customer  
**Storage**: PostgreSQL with multi-tenant isolation  
**Multi-tenancy**: Full (organization_id scoping)

---

## 🔧 Configuration Reference

### **Environment Variables**

| Variable | Required | Default | Purpose |
|----------|----------|---------|---------|
| `KUBEROOT_CLUSTER_ID` | No | `"local"` | Identifier for this cluster in diagnoses |
| `DATABASE_URL` | No | - | PostgreSQL connection string (enables production mode) |
| `KUBECONFIG` | No | `~/.kube/config` | Path to kubeconfig file |

### **Database URL Format**
```
postgres://username:password@host:port/database?sslmode=disable
```

**Example**:
```bash
DATABASE_URL='postgres://kuberoot:secretpass@db.example.com:5432/kuberoot?sslmode=require'
```

---

## 📝 Summary

**What Kuberoot Does**:
1. Watches Kubernetes pods for failures
2. Correlates failures with recent events
3. Matches patterns to known rules
4. Provides actionable diagnosis + suggested fixes
5. Stores history for pattern analysis

**Key Features**:
- ✅ Real-time failure detection
- ✅ Event correlation (last 3 events per failure)
- ✅ Confidence-based diagnosis
- ✅ Multi-tenant ready (via API keys)
- ✅ Historical analytics (PostgreSQL)
- ✅ Dual-mode: local dev + production

**Technology Stack**:
- **Language**: Go 1.25
- **K8s Client**: client-go v0.35.1
- **Database**: PostgreSQL 16
- **Auth**: SHA-256 API keys
- **HTTP**: Native net/http

**Why It Matters**:
Instead of:
```bash
kubectl get pods -A | grep -v Running
kubectl describe pod crashloop-demo -n debug-lab
kubectl logs crashloop-demo -n debug-lab
# 15 minutes of manual debugging
```

You get:
```bash
curl http://localhost:8080/diagnose | jq .
# Instant diagnosis with suggested fix
```

**Next Steps** (Future Enhancements):
- Add more rules (PVC failures, NodeNotReady, etc.)
- Pattern detection (recurring failures)
- Slack/PagerDuty notifications
- Web dashboard (React)
- AI-powered root cause suggestions
- Multi-cluster federation
