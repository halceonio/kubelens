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

> Note: KubeLens expects a `groups` claim in the access token. In Keycloak, add the **Group Membership** mapper (client scope `groups`) to the `kubelens` client and include the `groups` scope in the auth request.

## Log stream tuning
```yaml
logs:
  app_stream_resync_seconds: 10
```
This controls how often app log streams re-check pod membership to pick up new replicas or rolling updates.

## SSE timeouts
Log streaming uses long-lived SSE connections. Set:
```yaml
server:
  write_timeout_seconds: 0
```
to disable the write timeout so streams are not terminated mid-session.

## Local cache in the container
When using the single-container image, you can optionally start a local Valkey instance with:
```
START_LOCAL_VALKEY=true
LOCAL_VALKEY_DATA_DIR=/data/cache
LOCAL_VALKEY_MAXMEMORY=512mb
```
If no `LOCAL_VALKEY_MAXMEMORY` is provided, Valkey auto-tunes maxmemory to ~70% of the container’s memory limit when available.

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
  api_cache:
    enable_informers: true
    informer_resync_seconds: 30
    pod_list_ttl_seconds: 2
    app_list_ttl_seconds: 5
    crd_list_ttl_seconds: 10
    retry_attempts: 3
    retry_base_delay_ms: 200
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
