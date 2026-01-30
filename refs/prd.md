# PRD: KubeLens Backend MVP

**Date**: 2026-01-30
**Status**: Draft
**Owner**: TBD

## Summary
Build a secure, read-only backend that proxies the Kubernetes API for KubeLens. The backend enforces Keycloak group access, namespace allowlists, and resource filters, and provides log streaming, pod inspection, and metrics endpoints required by the frontend. AI/LLM features are explicitly out of scope.

## Goals
- Provide a stable, high-performance backend using Go + client-go (in-cluster only).
- Enforce access control via Keycloak JWT verification and group membership.
- Support log streaming with resume from last timestamp and configurable tail/line limits.
- Implement all endpoints specified in `refs/backend_ref.md` plus minimal additions required by the frontend.
- Support redis/sqlite/postgres as cache/storage backends for log buffers and terminated pod retention.

## Non-Goals
- No cluster mutation (create/update/delete resources).
- No AI/LLM log analysis.
- No out-of-cluster deployment support in MVP.
- No multi-cluster support in MVP.

## Personas
- **App Developer**: Needs quick access to pod logs and basic resource inspection.
- **Support Engineer**: Investigates incidents via logs across multiple replicas.
- **Security/Platform Team**: Requires strict RBAC enforcement and auditability.

## User Stories (MVP)
- As a developer, I can view logs for a pod or an app and resume streaming after a reconnect.
- As a developer, I can filter accessible resources by namespace and label/regex policies.
- As a support engineer, I can inspect pod details and basic resource metrics.
- As a platform admin, I can restrict access based on Keycloak groups and config.

## Functional Requirements

### 1) Authentication & Authorization
- Validate JWT signature and claims using Keycloak OIDC discovery and JWKS.
- Validate `aud` includes the configured client ID.
- Require membership in `k8s-logs-access` for all API access.
- Allow secret values only if user is in `k8s-admin-access` (or `allowed_secrets_groups`).

### 2) Configuration
- Load `config.yaml` on startup (in-cluster only).
- Must include: Keycloak settings, allowed namespaces, include/exclude filters, app group labels, cache/storage settings.
- Accept `auth.allowed_groups` as canonical, but support `auth.allows_groups` as a legacy alias for compatibility.
- Add log defaults:
  - `logs.default_tail_lines` (default 10000)
  - `logs.max_line_length` (default 10000)
  - `logs.max_tail_lines` (default 10000)
- `kubernetes.terminated_log_ttl` default: 3600 seconds (60 minutes).

### 3) Discovery Endpoints
- `GET /api/v1/namespaces` -> list from allowlist.
- `GET /api/v1/namespaces/{ns}/pods` -> filtered pods.
- `GET /api/v1/namespaces/{ns}/apps` -> filtered Deployments + StatefulSets.

### 4) Logging & Streaming
- `GET /api/v1/namespaces/{ns}/pods/{name}/logs`
  - Query: `tail`, `since`, `container`.
  - If `tail` is absent, use `logs.default_tail_lines`.
  - If `tail` exceeds `logs.max_tail_lines`, clamp to the max.
  - Support streaming (SSE preferred).
  - Reconnect should resume from last received timestamp using `since`.
  - Enforce max tail lines and max line length (truncate with a marker).
- For app views, merge logs from multiple pods into a single stream with pod prefix.
- `GET /api/v1/namespaces/{ns}/apps/{name}/logs` streams merged logs for an app (aggregated across pods).

### 5) Pod Details & Metrics
- `GET /api/v1/namespaces/{ns}/pods/{name}/details`
  - Mask secret values unless authorized.
- `GET /api/v1/namespaces/{ns}/pods/{name}/metrics`
  - Proxy Metrics Server data.

### 6) Filtering
- Apply include regex and exclude label filters for pods and apps.
- Enforce namespace allowlist for all endpoints.

### 7) Storage/Cache
- Support redis/sqlite/postgres backends.
- Store recent logs for terminated pods with 60-minute TTL.
- Provide an in-memory ring buffer fallback when cache is disabled.

### 8) Session Persistence
- Store and retrieve per-user UI session config keyed by `jwt.sub`.
- API:
  - `GET /api/v1/session` -> returns saved session (or empty/default).
  - `PUT /api/v1/session` -> upsert session payload (size-limited).
- `DELETE /api/v1/session` -> clear saved session for the user.
- Session payload should include: active resources, pinned resources, theme, and any other UI preferences currently stored in localStorage.
- The frontend must expose a manual “Clear Session” action to reset stored preferences.


### 9) Additional Endpoints (Frontend Integration)
- `GET /api/v1/namespaces/{ns}/pods/{name}` (direct fetch)
- `GET /api/v1/namespaces/{ns}/apps/{name}` (direct fetch)
- `GET /healthz` and `GET /readyz` (basic health checks)

## Non-Functional Requirements
- **Performance**: Handle large tails (10k lines) and long lines (10k chars) without UI stutter.
- **Security**: Zero trust to incoming requests; enforce group membership and namespace allowlist.
- **Reliability**: Log stream reconnect resumes without loss using timestamp cursor.
- **Observability**: Structured logs and basic metrics (request counts, latency).

## Dependencies
- Kubernetes API server + Metrics Server.
- Keycloak OIDC (realm + client configured).
- Redis/Postgres/SQLite for cache/storage (configurable).

## Risks
- Log streaming fan-out for app views can overload API without caching.
- Metrics Server may not be installed in some clusters.
- JWKS rotation requires cache invalidation and retry logic.

## Out of Scope
- AI/LLM analysis features.
- Write operations against Kubernetes resources.
- Multi-cluster routing.
