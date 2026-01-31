
export type LogLevel = 'INFO' | 'WARNING' | 'ERROR' | 'DEBUG';

export interface LogEntry {
  id: string;
  timestamp: string;
  level: LogLevel;
  message: string;
  podName: string;
  containerName: string;
  kind?: 'log' | 'marker';
  markerKind?: string;
}

export interface Container {
  name: string;
  image: string;
  ready: boolean;
  restartCount: number;
}

export interface VolumeMount {
  name: string;
  mountPath: string;
  readOnly: boolean;
}

export interface ResourceUsage {
  cpuUsage: string;
  cpuRequest: string;
  cpuLimit: string;
  memUsage: string;
  memRequest: string;
  memLimit: string;
}

export interface Pod {
  name: string;
  namespace: string;
  status: 'Running' | 'Pending' | 'Failed' | 'Succeeded' | 'Unknown';
  restarts: number;
  age: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  env: Record<string, string>;
  envSecrets?: string[];
  light?: boolean;
  metadataOnly?: boolean;
  containers: Container[];
  volumes: VolumeMount[];
  secrets: string[];
  configMaps: string[];
  resources: ResourceUsage;
  ownerApp?: string; // Links pod to its Deployment/StatefulSet
}

export interface AppResource {
  name: string;
  namespace: string;
  type: 'Deployment' | 'StatefulSet' | 'Cluster' | 'Dragonfly' | string;
  replicas: number;
  readyReplicas: number;
  podNames: string[];
  labels: Record<string, string>;
  annotations: Record<string, string>;
  env: Record<string, string>;
  envSecrets?: string[];
  resources: ResourceUsage;
  volumes: VolumeMount[];
  secrets: string[];
  configMaps: string[];
  containers?: Container[];
  image?: string; // Image tag used if version label is missing
  light?: boolean;
  metadataOnly?: boolean;
}

export interface SavedView {
  id: string;
  name: string;
  namespace?: string;
  labelRegex?: string;
  logLevel?: LogLevel | 'ALL';
  group?: string;
  autoApply?: boolean;
}

export interface ViewFilters {
  namespace?: string;
  labelRegex?: string;
  logLevel?: LogLevel | 'ALL';
  group?: string;
}

export interface LogViewPreferences {
  density?: 'default' | 'small' | 'smaller' | 'large' | 'larger';
  wrap?: boolean;
  show_timestamp?: boolean;
  show_details?: boolean;
  show_metrics?: boolean;
}

export interface ResourceIdentifier {
  type: 'pod' | 'app';
  namespace: string;
  name: string;
}

export interface Namespace {
  name: string;
}

export interface AppGroupConfig {
  enabled: boolean;
  labels: {
    selector: string;
    name: string;
    environment: string;
    version: string;
  };
}

export interface UiConfig {
  kubernetes: {
    cluster_name?: string;
    allowed_namespaces: string[];
    label_prefix: string;
    app_groups: AppGroupConfig;
  };
  logs: {
    default_tail_lines: number;
    max_tail_lines: number;
    max_line_length: number;
  };
}

export interface AuthUser {
  username: string;
  email: string;
  groups: string[];
  isAuthenticated: boolean;
  accessToken?: string;
  canViewSecrets?: boolean;
}
