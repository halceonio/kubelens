# Deployment Guide

## Container image
Build the container image:
```bash
docker build -f docker/Dockerfile -t halceon/kubelens .
```

## Runtime model (single pod)
The container runs:
- Nginx serving the frontend and proxying `/api/*` to the backend.
- Go backend listening on `:8080`.
- Supervisor managing both processes.

## Required config
Mount your config file and kubeconfig:
- `/etc/kubelens/config.yaml`
- `/etc/kubeconfig`

Environment variables:
- `KUBELENS_CONFIG=/etc/kubelens/config.yaml`
- `KUBECONFIG=/etc/kubeconfig`

## Example (docker compose)
```bash
docker compose -f docker/docker-compose.yml up --build
```

## Kubernetes notes
- Run the container as a single pod behind a Service.
- Mount the config file via ConfigMap and the kubeconfig via Secret.
- Keep the pod read-only: no cluster write permissions.

