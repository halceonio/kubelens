# Backend Runtime Environment

The backend is configured via YAML and a small set of environment variables that point to configuration and kubeconfig files.

## Required (production)
- `KUBELENS_CONFIG` or `KUBELENS_CONFIG_PATH`: absolute or relative path to the YAML config file.

## Optional (local / test)
- `KUBELENS_KUBECONFIG`: path to a kubeconfig file used for local development.
- `KUBECONFIG`: standard Kubernetes kubeconfig env var (used if `KUBELENS_KUBECONFIG` is not set).

## Optional (container cache bootstrap)
When running the all-in-one container, you can start a local Valkey (Redis-compatible) instance:
- `START_LOCAL_VALKEY` or `START_LOCAL_REDIS`: set to `true`, `1`, or `yes` to enable.
- `LOCAL_VALKEY_DATA_DIR` or `LOCAL_REDIS_DATA_DIR`: data directory for Valkey (default `/data/cache`).
- `LOCAL_VALKEY_MAXMEMORY` or `LOCAL_REDIS_MAXMEMORY`: optional max memory setting (e.g. `512mb` or bytes).

If a local cache is enabled, the entrypoint forces:
```yaml
cache:
  enabled: true
  redis_url: "redis://localhost:6379/0"
```

Maxmemory auto-tuning: if `LOCAL_*_MAXMEMORY` is not set and a container memory limit is detected, the entrypoint sets Valkey maxmemory to ~70% of the cgroup limit. If no limit is detected, Valkey runs without a maxmemory cap.

## Defaults
If no env vars are set, the backend will look for config files in this order:
1) `KUBELENS_CONFIG` or `KUBELENS_CONFIG_PATH`
2) `/etc/kubelens/config.yaml`
3) `./config.yaml`
4) `./backend/config.yaml`

If no kubeconfig is set, the backend defaults to in-cluster configuration.
