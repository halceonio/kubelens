
export type LogLevel = 'INFO' | 'WARNING' | 'ERROR' | 'DEBUG';

export interface LogEntry {
  id: string;
  timestamp: string;
  level: LogLevel;
  message: string;
  podName: string;
  containerName: string;
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
  status: 'Running' | 'Pending' | 'Failed' | 'Succeeded';
  restarts: number;
  age: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  env: Record<string, string>;
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
  type: 'Deployment' | 'StatefulSet';
  replicas: number;
  readyReplicas: number;
  podNames: string[];
  labels: Record<string, string>;
  annotations: Record<string, string>;
  env: Record<string, string>;
  resources: ResourceUsage;
  volumes: VolumeMount[];
  secrets: string[];
  configMaps: string[];
  image?: string; // Image tag used if version label is missing
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

export interface Config {
  keycloakUrl: string;
  realm: string;
  clientId: string;
  allowedNamespaces: string[];
  podFilterRegex: string;
  labelPrefix: string;
  appGroups: AppGroupConfig;
}

export interface AuthUser {
  username: string;
  email: string;
  groups: string[];
  isAuthenticated: boolean;
  accessToken?: string;
}
