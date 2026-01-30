# KubeLens Backend Integration Specification

## 1. Architectural Overview

The backend acts as a **secure proxy** between the KubeLens frontend and the Kubernetes API Server. It should be built using a language with strong K8s client support (Go with client-go, Rust with kube-rs, or Node.js with @kubernetes/client-node).

### Key Responsibilities:

- **Auth Validation:** Verify Keycloak JWT tokens and check for the `k8s-logs-access` group or as defined within the config file (i.e. `keycloak_allowed_groups`).
- **Resource Filtering:** Enforce the "Allowed Namespaces" and "Regex Filter" policies.
- **K8s Client:** Interact with the K8s API (In-cluster config or Kubeconfig).
- **Log Aggregation:** Merge streams from multiple pods (for App views).
    

---

## 2. Configuration Schema

The backend should load a config.yaml file on startup to define the access scope.


```yaml
auth:
  keycloak_url: "https://sso.enterprise.com"
  realm: "production"
  client_id: "kubelens-client"
  allows_groups: 
  - "k8s-logs-access"
  allowed_secrets_groups:
  - "k8s-admin-access"

storage:
  database_url: "" # support sqlite or postgres based on url
  # database_url: postgres://postgres:postgres@localhost:5432
  # database_url: sqlite://path/to/store.sql
  
cache:
  enabled: true
  redis_url: "" # support redis based caching for logs or fallback to on-disk sqlite
  # additional caching config to control aggressiveness and optimzie performance

kubernetes:
  cluster_name: "srv-cluster-east"
  terminated_log_ttl: 1800 # 30 mins before deleting those expired logs
  api: # kubernetes api client config
    burst: "" # Max burst for throttle of Kubernetes client (e.g., "200")
	qps: ""   # Queries per second limit for Kubernetes client (e.g., "100")
  
  allowed_namespaces: # Restrict frontend visibility
    - "payment-svc"
    - "inventory-svc"
    - "auth-svc"
  
  app_groups: # allow for app groups tab view
    enabled: true
    labels: 
        selector: "app.logging.k8s.io/group" # label to group by
        name: "app.logging.k8s.io/name" # optional label key to use for app display name; default to label.selector value if missing
        environment: "app.logging.k8s.io/environment" # additional label if present to show within each app group for each app. Would display as a pill next to the app's name
        version: "app.logging.k8s.io/version" # additional label if present to show within each app group for each app. Would display as a pill next to the app's name. If not present, would use the current image tag from the app spec such as "v1.2.3"

  pod_filters:
    include_regex: ".*"
    exclude_labels:
      - "component=istio"
      - "heritage=Helm"
  
  app_filters:
	include_regex: ".*"
    exclude_labels:
      - "component=istio"
      - "heritage=Helm"

  # Prefix for enterprise-specific metadata display
  label_prefix: "logger.app.k8s.io"
```

---

## 3. API Endpoints

### A. Discovery Endpoints

Used by the Sidebar to populate the navigation tree.

#### `GET /api/v1/namespaces`

- **Description:** Returns the list of namespaces defined in the `allowed_namespaces` config. 
- **Response:** `string[]`
    

#### `GET /api/v1/namespaces/{ns}/pods`

- **Description:** Lists pods in a specific namespace.
- **Filter logic:** Must apply the `pod_filters` regex and label selectors defined in config.
- **Response:** `Pod[]` (matching `types.ts`)
    

#### `GET /api/v1/namespaces/{ns}/apps`

- **Description:** Lists Deployments and StatefulSets. 
- **Filter logic:** Must apply the `app_filters` regex and label selectors defined in config.
- **Response:** `AppResource[]`
    

---

### B. Logging & Observation

The core of the application.

#### `GET /api/v1/namespaces/{ns}/pods/{name}/logs`

- **Query Params:**
    - `tail`: Number of lines (default 100).
    - `since`: Timestamp for incremental fetches.
    - `container`: Specific container name (optional).
        
- **Streaming (Recommended):** Use **Server-Sent Events (SSE)** or **WebSockets** to implement the `watch` functionality for real-time log tailing.
    

#### `GET /api/v1/namespaces/{ns}/pods/{name}/details`

- **Description:** Returns the full Pod spec including Env, Volumes, Secrets, and ConfigMaps.
- **Security Note:** The backend should mask sensitive Secret values unless the user has an additional `k8s-secrets-decrypt` group or as specified in the config.
    

---

### C. Resource Metrics

Optional but recommended for the "Overview" tab in the Inspector.

#### `GET /api/v1/namespaces/{ns}/pods/{name}/metrics`

- **Description:** Proxies to the K8s Metrics Server (`/apis/metrics.k8s.io/v1beta1/`).
- **Response:** Current CPU/Memory usage.
    

---

## 4. Authentication Flow

1. **Frontend:** User logs into Keycloak. The `AuthGuard` stores the `access_token`.
2. **Request:** Every API request includes `Authorization: Bearer <token>.
3. **Backend:**
    
    - Validates the token signature against Keycloak's JWKS endpoint.
    - Checks the `groups` claim in the JWT.
    - **Rejection:** If the token is invalid or the group is missing, return `403 Forbidden`.
        

---

## 5. Data Model Mapping

To maintain compatibility with the frontend, the backend should map raw K8s API responses to our simplified JSON structure:

|                        |                                                          |
| ---------------------- | -------------------------------------------------------- |
| **KubeLens Type**      | **K8s Source**                                           |
| `Pod.status`           | `pod.status.phase`                                       |
| `Pod.containers`       | `pod.spec.containers`                                    |
| `Pod.env`              | `pod.spec.containers[0].env` (mapped to key-value)       |
| `Pod.resources`        | `pod.spec.containers[0].resources` + Metrics Server data |
| `AppResource.replicas` | `deployment.status.replicas`                             |
| `AppResource.podNames` | Queried via `deployment.spec.selector`                   |

---

## 6. Implementation Notes for "Watch" Logic

For the **LogView** to feel like a real IDE:
- Use a **Log Buffer** in the backend (e.g., Redis or an in-memory ring buffer) if many users are viewing the same high-traffic pod logs.
- Implement **Log Multiplexing**: If a user selects an "App" view with 4 replicas, the backend should open 4 streams to K8s and merge them into a single SSE stream, prefixing each line with the Pod ID.
