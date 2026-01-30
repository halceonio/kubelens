# Configuration

KubeLens uses a YAML config file for backend behavior. See `backend/config.example.yaml` for the full schema.

## Highlights
- **Auth**: Keycloak OIDC with required group membership.
- **Logs**: Defaults to 10,000 tail lines with 10,000 character max line length.
- **Session storage**: Redis, sqlite, or postgres.
- **Kubernetes**: Namespace allowlist, app grouping labels, include/exclude filters.

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

