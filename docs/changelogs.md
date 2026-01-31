# Changelog

## v0.0.2 - 2026-01-31
- Saved views with namespace/label/group/level presets and per-user persistence.
- Global log search with highlight + jump, plus per-stream wrap/time/detail overrides.
- Stream health indicator with reconnect/lag stats and backpressure counters.
- Pod lifecycle markers inline (added/removed/ready/restart) for app streams.
- Container switcher for multi-container apps and log line annotations + permalinks.
- Log stream rate limiting and structured audit logging.
- Config validation endpoint (`/api/v1/config/validate`) and UI enhancements for stream sources.

## v0.0.1 - 2026-01-30
- Initial release of KubeLens MVP (read-only Kubernetes log analyzer).
- SSE log streaming with pooled workers and optional Redis Streams fan-out.
- App/pod list caching (informers + TTL) with metadata-only mode.
- Keycloak SSO with group-based access control and masked secrets.
- Server-side session persistence (redis/sqlite/postgres).
- Docker all-in-one runtime with nginx + supervisor and optional local Valkey.
- Helm chart + Kustomize manifests for in-cluster deployment.
- MkDocs Material documentation site with deployment guides.
