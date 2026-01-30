# KubeLens - Enterprise Log Analyzer

KubeLens is an internal, read-only Kubernetes log analyzer designed for non-infrastructure teams. It provides a focused interface for viewing, filtering, and inspecting pod logs without exposing full cluster management capabilities.

> Note: AI/LLM features are intentionally out of scope for this project.

## Why KubeLens
Many enterprise teams need quick insight into application behavior but should not require direct `kubectl` access or broad IDE permissions. KubeLens narrows the experience to the critical debugging path: logs and resource inspection.

## Key features
- Multi-pane log dashboard (up to 4 concurrent streams)
- Virtualized log rendering for high-volume streams
- Log level filtering, regex search, auto-scroll, and line wrapping
- Pod inspector for env/config/secrets (masked) and resource limits/usage
- App grouping via custom labels, namespace allowlist, and pinned resources
- Keycloak SSO with RBAC gate on `k8s-logs-access`

## Tech stack
- React 18.3.1 + TypeScript
- Vite
- Tailwind CSS
- `react-window` for virtualization

## Repo structure
- `frontend/` UI application
- `refs/` product and backend reference docs

## Local development
```bash
cd frontend
npm install
npm run dev
```

## Backend integration (spec)
The backend is expected to act as a secure proxy to the Kubernetes API. The full reference spec and configuration schema are in `refs/backend_ref.md`. The frontend currently uses mocked data and can be wired to real endpoints later.

### Example configuration
```yaml
auth:
  keycloak_url: "https://sso.enterprise.com"
  realm: "production"
  client_id: "kubelens-client"
  allows_groups:
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

## License
Internal use only.
