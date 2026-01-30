# Architecture

KubeLens runs as a single pod (frontend + backend + nginx) and exposes a single HTTP entrypoint.

## Components
- **Frontend**: React + Vite SPA served by nginx.
- **Backend**: Go service that handles authentication, session storage, and Kubernetes API proxying.
- **Nginx**: Serves static assets and proxies `/api/*` requests to the backend.
- **Supervisor**: Manages the nginx and backend processes inside the container.

## Data flow
1) User loads the SPA from nginx.
2) SPA redirects to Keycloak for auth if needed.
3) Backend validates JWTs and applies namespace/label filters.
4) Logs stream via SSE from the backend to the frontend.
5) User preferences persist via the backend session store.

## Auth config handshake
The frontend loads Keycloak settings at runtime from `GET /api/v1/auth/config`.
The response is cached locally for a few minutes to reduce repeated calls, and
the UI only falls back to build-time `VITE_KEYCLOAK_*` overrides if the backend
endpoint is unavailable.
