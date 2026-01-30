# KubeLens Documentation

KubeLens is a read-only Kubernetes log analyzer that provides a focused UI for logs and resource inspection without full cluster management access.

## Contents
- [Quickstart](quickstart.md)
- [Backend Runtime Environment](runtime-env.md)
- [Deployment Guide](deploy.md)
- [Configuration](configuration.md)
- [Architecture](architecture.md)
- [Screenshots](screenshots.md)
- [GitHub Pages Publishing](github-pages.md)

## Project Layout
- `frontend/` React + Vite UI
- `backend/` Go API server (auth, sessions, Kubernetes proxy)
- `docker/` Dockerfile, nginx, supervisor, and compose assets
- `refs/` product and backend specs
