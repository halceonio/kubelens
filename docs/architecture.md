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

