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

## Shared log workers (Redis Streams)
KubeLens can pool log streams across multiple backend replicas using Redis Streams:
```yaml
logs:
  use_redis_streams: true
  redis_stream_prefix: "kubelens:logs"
  redis_stream_maxlen: 10000
  redis_stream_block_millis: 2000
  redis_lock_ttl_seconds: 15
  redis_url: "" # optional override; defaults to cache.redis_url
```
When enabled, each pod/container log stream is handled by a single leader that writes to Redis, while other replicas read and fan out to their subscribers. This reduces upstream Kubernetes log streams and improves team-scale usage.
The in-process log worker also maintains a ring buffer (default 10k lines) to support fast replay for reconnecting clients.

## Log stream rate limiting
To avoid excessive log stream opens per user/namespace:
```yaml
logs:
  rate_limit_per_minute: 120
  rate_limit_burst: 240
```
Limits apply per user + namespace and return `429` when exceeded.

## Audit logging
Enable structured audit logs for key actions (logs, reads, lists):
```yaml
server:
  audit_logs: true
```

## Custom resources
You can add additional CRDs to the Apps view via config:
```yaml
kubernetes:
  custom_resources:
    - name: "cnpg"
      group: "postgresql.cnpg.io"
      version: "v1"
      resource: "clusters"
      kind: "Cluster"
      enabled: true
      pod_label_key: "cnpg.io/cluster"
```
`pod_label_key` is optional but required for log streaming to work for the CRD.

## API cache modes
The API cache supports an optional metadata-only list mode to reduce API server load:
```yaml
kubernetes:
  api_cache:
    metadata_only: true
```
When enabled, list endpoints return `metadataOnly: true` resources with minimal fields, and the UI fetches full resource details on demand.

Cache metrics are exposed at:
```
GET /api/v1/metrics
```

Configuration validation is available at:
```
GET /api/v1/config/validate
```

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
    metadata_only: false
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
