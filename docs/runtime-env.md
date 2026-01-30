# Backend Runtime Environment

The backend is configured via YAML and a small set of environment variables that point to configuration and kubeconfig files.

## Required (production)
- `KUBELENS_CONFIG` or `KUBELENS_CONFIG_PATH`: absolute or relative path to the YAML config file.

## Optional (local / test)
- `KUBELENS_KUBECONFIG`: path to a kubeconfig file used for local development.
- `KUBECONFIG`: standard Kubernetes kubeconfig env var (used if `KUBELENS_KUBECONFIG` is not set).

## Defaults
If no env vars are set, the backend will look for config files in this order:
1) `KUBELENS_CONFIG` or `KUBELENS_CONFIG_PATH`
2) `/etc/kubelens/config.yaml`
3) `./config.yaml`
4) `./backend/config.yaml`

If no kubeconfig is set, the backend defaults to in-cluster configuration.

