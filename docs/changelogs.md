# Changelog

## v0.0.1 - 2026-01-30
- Initial release of KubeLens MVP (read-only Kubernetes log analyzer).
- SSE log streaming with pooled workers and optional Redis Streams fan-out.
- App/pod list caching (informers + TTL) with metadata-only mode.
- Keycloak SSO with group-based access control and masked secrets.
- Server-side session persistence (redis/sqlite/postgres).
- Docker all-in-one runtime with nginx + supervisor and optional local Valkey.
- Helm chart + Kustomize manifests for in-cluster deployment.
- MkDocs Material documentation site with deployment guides.
