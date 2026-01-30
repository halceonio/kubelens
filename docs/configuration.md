# Configuration

KubeLens uses a YAML config file for backend behavior. See `backend/config.example.yaml` for the full schema.

## Highlights
- **Auth**: Keycloak OIDC with required group membership.
- **Logs**: Defaults to 10,000 tail lines with 10,000 character max line length. App log streams resync pod membership every 10s by default.
- **Session storage**: Redis, sqlite, or postgres.
- **Kubernetes**: Namespace allowlist, app grouping labels, include/exclude filters.

## Frontend auth config
The frontend reads Keycloak settings at runtime from:
```
GET /api/v1/auth/config
```
This endpoint returns the Keycloak URL, realm, client ID, and allowed groups from the backend config. It does **not** return secrets. The UI caches this response locally for a few minutes and will only fall back to build-time `VITE_KEYCLOAK_*` overrides if the endpoint is unavailable.

## Log stream tuning
```yaml
logs:
  app_stream_resync_seconds: 10
```
This controls how often app log streams re-check pod membership to pick up new replicas or rolling updates.

## Example
```yaml
auth:
  keycloak_url: "https://keycloak.enterprise.com"
  realm: "monitoring"
  client_id: "kubelens"
  client_secret: "REDACTED"
  allowed_groups:
    - "k8s-logs-access"
  allowed_secrets_groups:
    - "k8s-admin-access"

storage:
  database_url: "sqlite://./data/kubelens.sqlite"

cache:
  enabled: true
  redis_url: "redis://localhost:6379/0"

kubernetes:
  cluster_name: "enterprise-cluster"
  allowed_namespaces:
    - "apps"
    - "db"
  app_groups:
    enabled: true
    labels:
      selector: "app.enterprise.com/name"
      name: "app.enterprise.com/displayname"
      environment: "app.enterprise.com/env"
      version: "app.enterprise.com/version"
```
