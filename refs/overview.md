# KubeLens - Enterprise Log Analyzer

KubeLens is a specialized, internal enterprise web application designed to empower non-infrastructure and development teams with a "slimmed-down" Kubernetes IDE experience. It focuses on the critical path of application debugging: viewing, searching, and analyzing pod logs without the complexity of a full-scale cluster management tool.

## 🚀 Purpose
In many enterprise environments, giving full `kubectl` access or complex IDE permissions to every developer can be a security risk or an overwhelming experience. KubeLens bridges this gap by providing a safe, read-only interface focused specifically on application health and troubleshooting.

## ✨ Key Features

### 🖥️ Multi-Pane Log Dashboard
*   **Grid Layout**: View up to 4 concurrent pod log streams in a responsive grid.
*   **Virtualized Rendering**: Powered by `react-window` to handle thousands of log lines with 60fps performance.
*   **Log Controls**: Real-time filtering by log level (Info, Warn, Error), regex search, auto-scroll, and line wrapping.

### 🔍 Deep Resource Inspection
*   **Pod Inspector**: View environment variables, secrets, configmaps, and resource limits (CPU/Memory) in a clean, tabbed interface.
*   **Real-time Metrics**: Visual progress bars for resource allocation (Request vs. Limit).

### 📂 App Groups & Navigation
*   **App Grouping**: Group resources by custom enterprise labels (e.g., `app.logging.k8s.io/group`) instead of just namespaces. Optional label keys can provide display names per app.
*   **Namespace Filtering**: Restricts access to a whitelist of namespaces defined in the global configuration.
*   **Pinned Resources**: Save frequently accessed pods or apps to a sidebar for quick access.

### 🔐 Enterprise Security
*   **Keycloak Integration**: SSO-ready authentication.
*   **RBAC Enforcement**: Hard requirement for the `k8s-logs-access` group membership to enter the app.

## 🛠️ Configuration
KubeLens is configured via a central configuration schema (mocked in `constants.tsx` and defined in `types.ts`):

```yaml
kubernetes:
  allowed_namespaces: ["payment-svc", "auth-svc"]
  app_groups:
    enabled: true
    labels:
      selector: "app.logging.k8s.io/group"
      environment: "app.logging.k8s.io/environment"
      version: "app.logging.k8s.io/version"
```

## 🏗️ Technical Architecture
*   **Frontend**: React 18.3.1 (downgraded for stable virtualization compatibility).
*   **Styling**: Tailwind CSS with a custom manual Light/Dark mode toggle.
*   **Virtualization**: `react-window` for high-performance log rendering.
*   **Persistence**: Session state is stored via the backend per user, with URL fragments and localStorage used as fallbacks.

## 🎨 Design Philosophy
KubeLens follows a "Developer First" aesthetic: high-contrast typography, JetBrains Mono for log data, and a dense, information-rich UI that minimizes clicking while maximizing visibility into system state.
