# Speckit: KubeLens Backend MVP Implementation Plan

**Date**: 2026-01-30
**Target**: MVP backend that powers the existing frontend UX

## Summary
Implement a Go-based, in-cluster backend that securely proxies the Kubernetes API and exposes log, discovery, and inspection endpoints with strict RBAC and filtering. Use SSE for log streaming with resume-by-timestamp. Support redis/sqlite/postgres for log buffering and terminated-pod retention.

## Architecture Decisions
- **Language**: Go (highest performance + stability) with `client-go` for Kubernetes API access.
- **Deployment**: In-cluster only (service account + RBAC).
- **Streaming**: Server-Sent Events (SSE) for log tailing (one-way, simple, HTTP-friendly).
- **Auth**: Keycloak JWT validation via OIDC discovery and JWKS caching.
- **Storage/Cache**: Pluggable interface with redis/sqlite/postgres implementations.

## Data Flow (High-Level)
1. Request enters API -> auth middleware validates JWT + group membership.
2. Namespace allowlist and label/regex filters applied.
3. K8s client queries resources or streams logs.
4. Logs are streamed via SSE with event IDs (timestamps), optional cache buffer.
5. Responses mapped to frontend data models.

## Implementation Plan

### Sprint 0: Scaffold + Config + Health
**Goal**: Bootstrapped backend with config loading and basic health endpoints.
**Deliverable**: Runnable server that reads config and serves `/healthz` and `/readyz`.

Tasks:
1) **Project skeleton**
- Create `backend/` Go module.
- Suggested layout:
  - `backend/cmd/server/main.go`
  - `backend/internal/config/`
  - `backend/internal/api/`
  - `backend/internal/auth/`
  - `backend/internal/k8s/`
  - `backend/internal/storage/`
  - `backend/internal/logs/`
  - `backend/internal/filters/`
- Define build/run scripts.

2) **Config loader**
- Parse `config.yaml` and apply defaults (tail lines, max line length, TTL).
- Validate required fields (Keycloak base URL, realm, client ID, namespaces).
- Support `auth.allowed_groups` as canonical and accept `auth.allows_groups` as legacy alias.

3) **Health endpoints**
- Implement `/healthz` and `/readyz`.
- Readiness checks: config loaded, K8s in-cluster config obtainable.

**Validation**:
- Server starts and serves health endpoints.
- Config defaults applied correctly.

---

### Sprint 0.5: Session Persistence API
**Goal**: Persist and restore per-user UI session config keyed by `jwt.sub`.
**Deliverable**: `GET /api/v1/session` and `PUT /api/v1/session` backed by storage layer.

Tasks:
1) **Session model**
- Define session payload schema (active resources, pinned resources, theme, UI prefs).
- Set size limits and validation rules.

2) **Storage integration**
- Implement session storage in redis/sqlite/postgres backends.
- Key by `jwt.sub` and enforce TTL if required.

3) **Endpoints**
- `GET /api/v1/session` returns saved session or empty/default.
- `PUT /api/v1/session` upserts payload for current user.
- `DELETE /api/v1/session` clears stored session.

**Validation**:
- Session is saved and returned for same JWT `sub`.
- Payload validation rejects oversized/invalid data.
- Delete clears the session and returns empty/default on next GET.

---

### Sprint 1: Auth + Discovery APIs
**Goal**: Secure endpoints and resource discovery.
**Deliverable**: Authenticated API for namespaces, pods, and apps.

Tasks:
1) **Auth middleware**
- Fetch OIDC metadata (`.well-known/openid-configuration`) to obtain `jwks_uri`.
- Cache JWKS keys and validate JWT signature, expiry, issuer, and audience.
- Enforce group membership (`k8s-logs-access`).

2) **K8s client**
- Use `rest.InClusterConfig()` and `kubernetes.NewForConfig()`.
- Configure QPS/burst from config.

3) **Filtering utilities**
- Implement include/exclude regex and label exclusion matching for pods/apps.
- Enforce namespace allowlist at all endpoints.

4) **Discovery endpoints**
- `GET /api/v1/namespaces`
- `GET /api/v1/namespaces/{ns}/pods`
- `GET /api/v1/namespaces/{ns}/apps`
- Map K8s objects to frontend types.

**Validation**:
- Requests without valid JWT rejected (403/401).
- Discovery endpoints return filtered results.

---

### Sprint 2: Logs + Streaming + Resume
**Goal**: Core log streaming with tail/line limits and resume support.
**Deliverable**: SSE log endpoints for pods and app aggregation.

Tasks:
1) **Pod log endpoint**
- Implement `GET /api/v1/namespaces/{ns}/pods/{name}/logs`.
- Use `PodLogs` + `PodLogOptions` with `TailLines`, `SinceTime`, `Timestamps`, `Follow`.
- Enforce max tail lines and max line length (truncate with marker).
- Parse `since` (RFC3339) or `Last-Event-ID` to set `SinceTime` for resume.

2) **SSE streaming**
- Stream as `text/event-stream` with `id` set to log timestamp.
- Support reconnect by accepting `since` (RFC3339) or `Last-Event-ID`.

3) **App log aggregation**
- Merge streams from multiple pods into one SSE stream.
- Prefix log lines with pod identifier.
- Expose `GET /api/v1/namespaces/{ns}/apps/{name}/logs`.

4) **Additional lookup endpoints**
- `GET /api/v1/namespaces/{ns}/pods/{name}`
- `GET /api/v1/namespaces/{ns}/apps/{name}`

**Validation**:
- SSE reconnection resumes from last timestamp.
- Tail defaults and limits enforced (10k lines, 10k chars).

---

### Sprint 3: Pod Details + Metrics + Caching
**Goal**: Inspector data + caching for terminated pods.
**Deliverable**: Details/metrics endpoints and pluggable storage backends.

Tasks:
1) **Pod details**
- `GET /api/v1/namespaces/{ns}/pods/{name}/details`.
- Mask secrets unless user has `k8s-admin-access`.

2) **Metrics**
- `GET /api/v1/namespaces/{ns}/pods/{name}/metrics`.
- Query Metrics Server; handle absence gracefully.

3) **Storage/cache interface**
- Define storage interface for log buffers and terminated pod retention.
- Implement redis/sqlite/postgres backends.
- Enforce `terminated_log_ttl` (60 minutes).

4) **Log buffer integration**
- Use cache for fan-out across viewers and app log aggregation.

**Validation**:
- Inspector shows masked/unmasked secrets based on group.
- Logs for terminated pods remain accessible for TTL.

---

## Testing Strategy
- Unit tests: auth claims parsing, regex/label filters, log truncation.
- Integration tests: in-cluster config detection (mocked), JWT validation (JWKS stub).
- Manual validation: SSE reconnect with `since` timestamp; metrics endpoint when server available.

## Risks & Mitigations
- **High log volume**: enforce tail/line limits and buffer size.
- **JWKS rotation**: cache with TTL and refresh on signature failure.
- **Metrics server missing**: return 404 or empty metrics with clear error.
- **App aggregation load**: use buffer/cache to reduce API pressure.

## Rollback Plan
- Deploy behind feature flag or separate service name.
- Roll back to static/mock data on the frontend if backend fails.
