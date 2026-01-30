# KubeLens Agent Notes

## Project intent
KubeLens is a read-only, enterprise log analyzer that gives non-infra teams a slimmed-down Kubernetes IDE focused on pod logs and resource inspection. Keep the experience fast, safe, and log-centric. Do not add or document AI/LLM features.

## Repo layout
- `frontend/` React + Vite UI (TypeScript)
- `backend/` Go API server (auth, sessions, Kubernetes proxy, SSE streaming)
- `docker/` Dockerfile, nginx, supervisor, compose assets
- `docs/` MkDocs documentation site
- `charts/` Helm chart (namespace: `monitoring`)
- `deploy/` Kustomize manifests (namespace: `monitoring`)
- `refs/` product and backend reference notes

## Local dev
- `cd frontend`
- `npm install`
- `npm run dev`
 
Backend (separate shell):
- `cp backend/config.example.yaml backend/config.yaml`
- `export KUBELENS_CONFIG=backend/config.yaml`
- `export KUBECONFIG=~/.kube/config`
- `go run ./backend/cmd/server`

## Frontend architecture
- Core pages are composed in `frontend/App.tsx` with `Sidebar`, `LogView`, and `PodInspector`.
- Log rendering uses `react-window` for virtualization; keep log performance in mind when refactoring.
- Theme is a light/dark toggle stored in `localStorage` (`kubelens_theme`).
- Active tabs and pinned resources persist via `localStorage` and URL hash (`view=`).

## Data and API assumptions
- The UI currently uses mocked data in `frontend/constants.tsx` and `frontend/services/k8sService.ts`.
- The intended backend is a secure proxy to the Kubernetes API that enforces namespace allowlists and RBAC.
- Target endpoints and config schema are documented in `refs/backend_ref.md`. Align any frontend API wiring to that spec.
- Log streaming uses SSE with shared log workers. When `logs.use_redis_streams` is enabled, Redis Streams coordinates pooled log workers across replicas.
- List endpoints support `?light=true` and optional metadata-only mode for reduced K8s API load.

## Security expectations
- Authentication is Keycloak-based and requires membership in the `k8s-logs-access` group, or as defined in config.
- The UI is read-only; do not introduce mutating actions on cluster resources.

## Coding conventions
- React 18.3.1 compatibility is required for current virtualization tooling.
- Prefer small, focused components and keep log rendering virtualized.
- Avoid adding new dependencies unless necessary.
