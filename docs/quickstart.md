# Quickstart

## Local development
1) Configure the backend:
```bash
cp backend/config.example.yaml backend/config.yaml
export KUBELENS_CONFIG=backend/config.yaml
export KUBECONFIG=~/.kube/config
```

2) Configure the frontend:
```bash
cp frontend/.env.example frontend/.env
```

3) Run both services:
```bash
make dev
```

4) Open the UI:
```
http://localhost:3000
```

## Docker (single-container runtime)
```bash
cp backend/config.example.yaml docker/config.yaml
# Place a kubeconfig at docker/kubeconfig for local testing
docker compose -f docker/docker-compose.yml up --build
```

Open:
```
http://localhost:8080
```

