# KubeLens - Enterprise Log Analyzer

KubeLens is an internal, read-only Kubernetes log analyzer designed for non-infrastructure teams. It provides a focused interface for viewing, filtering, and inspecting pod logs without exposing full cluster management capabilities.

> Note: AI/LLM features are intentionally out of scope for this project.

## Why KubeLens
Many enterprise teams need quick insight into application behavior but should not require direct `kubectl` access or broad IDE permissions. KubeLens narrows the experience to the critical debugging path: logs and resource inspection.

## Key features
- Multi-pane log dashboard (up to 4 concurrent streams) with SSE streaming
- Virtualized log rendering for high-volume streams
- Log level filtering, regex search, auto-scroll, and line wrapping
- Pod inspector for env/config/secrets (masked) and resource limits/usage
- App grouping via custom labels, namespace allowlist, and pinned resources
- Keycloak SSO with RBAC gate on `k8s-logs-access`
- Server-side session persistence (redis/sqlite/postgres) keyed by `jwt.sub`
- App catalog supports Deployments, StatefulSets, CNPG Clusters, and Dragonfly CRDs

## Tech stack
- React 18.3.1 + TypeScript
- Vite
- Tailwind CSS
- `react-window` for virtualization
- Go backend (SSE, session storage, auth enforcement)
- Nginx + supervisor for in-cluster single-pod runtime

## Repo structure
- `frontend/` UI application
- `backend/` API server (auth, sessions, Kubernetes proxy)
- `docker/` runtime assets (Dockerfile, nginx, supervisor, compose)
- `refs/` product and backend reference docs
- `docs/` published documentation (GitHub Pages)

## Quickstart (local dev)
1) Configure the backend:
```bash
cp backend/config.example.yaml backend/config.yaml
export KUBELENS_CONFIG=backend/config.yaml
export KUBECONFIG=~/.kube/config
```
2) Configure the frontend:
```bash
cp frontend/.env.example frontend/.env
# Set VITE_KEYCLOAK_URL, VITE_KEYCLOAK_REALM, VITE_KEYCLOAK_CLIENT_ID
```
3) Run both services:
```bash
make dev
```
4) Open the UI:
```
http://localhost:3000
```

## Quickstart (Docker)
```bash
cp backend/config.example.yaml docker/config.yaml
# Place a kubeconfig at docker/kubeconfig for local testing
docker compose -f docker/docker-compose.yml up --build
```
Then open:
```
http://localhost:8080
```

The compose file expects `docker/config.yaml` and `docker/kubeconfig` to be mounted. You can override with `KUBELENS_CONFIG` and `KUBECONFIG`.

### Local Valkey (optional)
To run a local Redis-compatible cache inside the container:
```bash
START_LOCAL_VALKEY=true
LOCAL_VALKEY_DATA_DIR=/data/cache
LOCAL_VALKEY_MAXMEMORY=512mb
```
If `LOCAL_VALKEY_MAXMEMORY` is omitted, the container auto-tunes Valkey maxmemory to ~70% of the detected memory limit.

## Backend integration
The backend acts as a secure proxy to the Kubernetes API and enforces namespace allowlists and label filters. The full reference spec and configuration schema are in `refs/backend_ref.md`.

### Example configuration
```yaml
auth:
  keycloak_url: "https://sso.enterprise.com"
  realm: "production"
  client_id: "kubelens-client"
  allowed_groups:
    - "k8s-logs-access"
  allowed_secrets_groups:
    - "k8s-admin-access"

storage:
  database_url: "" # sqlite or postgres

cache:
  enabled: true
  redis_url: ""

kubernetes:
  cluster_name: "srv-cluster-east"
  terminated_log_ttl: 1800
  api:
    burst: "200"
    qps: "100"
  api_cache:
    enable_informers: true
    informer_resync_seconds: 30
    pod_list_ttl_seconds: 2
    app_list_ttl_seconds: 5
    crd_list_ttl_seconds: 10
    retry_attempts: 3
    retry_base_delay_ms: 200
    metadata_only: false
  allowed_namespaces:
    - "payment-svc"
    - "inventory-svc"
    - "auth-svc"
  app_groups:
    enabled: true
    labels:
      selector: "app.logging.k8s.io/group"
      name: "app.logging.k8s.io/name"
      environment: "app.logging.k8s.io/environment"
      version: "app.logging.k8s.io/version"
  pod_filters:
    include_regex: ".*"
    exclude_labels:
      - "component=istio"
      - "heritage=Helm"
  app_filters:
    include_regex: ".*"
    exclude_labels:
      - "component=istio"
      - "heritage=Helm"
  label_prefix: "logger.app.k8s.io"
```

## Configuration notes
- `logs.default_tail_lines`, `logs.max_tail_lines`, and `logs.max_line_length` default to `10000`.
- For SSE log streaming, set `server.write_timeout_seconds: 0` to avoid premature disconnects.
- Session persistence supports redis, sqlite, or postgres. With no storage configured, the in-memory store is used.
- For local testing, `KUBELENS_KUBECONFIG` or `KUBECONFIG` can point to a kubeconfig file.
- Set `kubernetes.api_cache.metadata_only: true` to return lightweight metadata lists (full details are fetched on demand).
- Prometheus-style cache metrics are exposed at `GET /api/v1/metrics`.
- Shared log workers can be enabled with `logs.use_redis_streams: true` (requires `cache.redis_url` or `logs.redis_url`).

## Documentation
- `docs/index.md` entry point
- `docs/runtime-env.md` backend runtime environment variables
- `docs/deploy.md` deployment guide
- `docs/github-pages.md` publishing instructions
- Preview docs locally: `make docs-preview`
- Helm chart: `charts/kubelens`
- Kustomize manifests: `deploy/`

## License
Internal use only.
