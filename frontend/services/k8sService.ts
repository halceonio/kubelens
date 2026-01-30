
import { Pod, LogEntry, LogLevel, AppResource, Namespace } from '../types';
import { MOCK_PODS, MOCK_NAMESPACES, USE_MOCKS } from '../constants';

const API_BASE = '/api/v1';

const buildHeaders = (token?: string | null) => {
  if (!token) return {};
  return { Authorization: `Bearer ${token}` };
};

const fetchJSON = async <T>(url: string, token?: string | null): Promise<T> => {
  const res = await fetch(url, { headers: buildHeaders(token) });
  if (!res.ok) {
    throw new Error(`Request failed: ${res.status}`);
  }
  return res.json();
};

const withRevealSecrets = (url: string, reveal?: boolean) => {
  if (!reveal) return url;
  const joiner = url.includes('?') ? '&' : '?';
  return `${url}${joiner}reveal_secrets=true`;
};

const generateMockLog = (podName: string, containerName: string, minutesOffset: number = 0): LogEntry => {
  const levels: LogLevel[] = ['INFO', 'INFO', 'INFO', 'WARNING', 'ERROR'];
  const messages = [
    "Successfully connected to database pool",
    "Processing incoming request for /api/v1/resource",
    "Cache miss for key: user_profile_123",
    "Memory usage approaching 85% threshold",
    "Unexpected termination of worker thread 4",
    "Request timeout from external service: inventory-api",
    "Environment variable 'DEBUG' is set to true",
    "Validation failed for input: { id: null }",
    "Heartbeat signal sent to master node",
    "Buffer overflow detected in stream handler",
    "New configuration reloaded from ConfigMap",
  ];
  
  const date = new Date();
  date.setMinutes(date.getMinutes() - minutesOffset);

  return {
    id: Math.random().toString(36).substr(2, 9),
    timestamp: date.toISOString(),
    level: levels[Math.floor(Math.random() * levels.length)],
    message: messages[Math.floor(Math.random() * messages.length)],
    podName,
    containerName
  };
};

export const getNamespaces = async (token?: string | null): Promise<Namespace[]> => {
  if (!token) {
    if (USE_MOCKS) return MOCK_NAMESPACES;
    throw new Error('Missing access token');
  }

  try {
    return await fetchJSON<Namespace[]>(`${API_BASE}/namespaces`, token);
  } catch (err) {
    console.warn('Failed to load namespaces from backend', err);
    if (USE_MOCKS) return MOCK_NAMESPACES;
    throw err;
  }
};

export const getPods = async (
  namespace: string,
  token?: string | null,
  opts?: { light?: boolean }
): Promise<Pod[]> => {
  if (!token && !USE_MOCKS) {
    throw new Error('Missing access token');
  }

  if (token) {
    try {
      const light = opts?.light ?? true;
      const url = `${API_BASE}/namespaces/${namespace}/pods${light ? '?light=true' : ''}`;
      return await fetchJSON<Pod[]>(url, token);
    } catch (err) {
      console.warn('Failed to load pods from backend', err);
      if (!USE_MOCKS) throw err;
    }
  }

  if (!USE_MOCKS) return [];

  return new Promise((resolve) => {
    setTimeout(() => {
      const basePods = (MOCK_PODS[namespace] || []) as any[];
      const fullPods: Pod[] = basePods.map(p => ({
        ...p,
        containers: [
          { name: 'main-app', image: 'enterprise/app:v1.2.3', ready: true, restartCount: p.restarts },
          { name: 'istio-proxy', image: 'istio/proxyv2:1.15.0', ready: true, restartCount: 0 },
        ],
        volumes: [
          { name: 'config-vol', mountPath: '/etc/config', readOnly: true },
          { name: 'data-vol', mountPath: '/var/lib/data', readOnly: false },
        ],
        secrets: ['db-credentials', 'api-keys', 'tls-certs'],
        configMaps: ['app-settings', 'feature-flags', 'nginx-conf'],
        env: {
          'DB_HOST': 'psql-cluster-01',
          'API_KEY': '********',
          'MAX_RETRIES': '5',
          'NODE_NAME': 'worker-node-04'
        },
        envSecrets: ['API_KEY'],
        resources: {
          cpuUsage: (Math.random() * 200).toFixed(0) + 'm',
          cpuRequest: '100m',
          cpuLimit: '500m',
          memUsage: (Math.random() * 512 + 128).toFixed(0) + 'Mi',
          memRequest: '256Mi',
          memLimit: '1Gi'
        }
      }));
      resolve(fullPods);
    }, 500);
  });
};

export const getPodByName = async (
  namespace: string,
  name: string,
  token?: string | null,
  opts?: { revealSecrets?: boolean }
): Promise<Pod | null> => {
  if (token) {
    try {
      const url = withRevealSecrets(`${API_BASE}/namespaces/${namespace}/pods/${name}`, opts?.revealSecrets);
      return await fetchJSON<Pod>(url, token);
    } catch (err) {
      console.warn('Failed to load pod from backend', err);
    }
  }
  const pods = await getPods(namespace, token, { light: false });
  return pods.find(p => p.name === name) || null;
};

export const getApps = async (
  namespace: string,
  token?: string | null,
  opts?: { light?: boolean }
): Promise<AppResource[]> => {
  if (!token && !USE_MOCKS) {
    throw new Error('Missing access token');
  }

  if (token) {
    try {
      const light = opts?.light ?? true;
      const url = `${API_BASE}/namespaces/${namespace}/apps${light ? '?light=true' : ''}`;
      return await fetchJSON<AppResource[]>(url, token);
    } catch (err) {
      console.warn('Failed to load apps from backend', err);
      if (!USE_MOCKS) throw err;
    }
  }

  if (!USE_MOCKS) return [];

  return new Promise((resolve) => {
    setTimeout(() => {
      const apps: AppResource[] = [
        {
          name: `${namespace}-api`,
          namespace,
          type: 'Deployment',
          replicas: 3,
          readyReplicas: 3,
          podNames: [
            `${namespace}-api-pod-1`, 
            `${namespace}-api-pod-2`, 
            `${namespace}-api-pod-3`,
            `${namespace}-api-old-pod-terminated`
          ],
          labels: { 
            'app': `${namespace}-api`, 
            'tier': 'frontend',
            'app.logging.k8s.io/group': namespace.includes('payment') ? 'Core Banking' : 'Support Systems',
            'app.logging.k8s.io/environment': 'production',
            'app.logging.k8s.io/version': 'v2.1.0'
          },
          annotations: { 'deployment.kubernetes.io/revision': '5' },
          env: { 'LOG_LEVEL': 'DEBUG', 'NODE_ENV': 'production' },
          envSecrets: [],
          volumes: [{ name: 'api-storage', mountPath: '/data', readOnly: false }],
          secrets: ['api-key-secret'],
          configMaps: ['api-config'],
          resources: {
            cpuUsage: '450m',
            cpuRequest: '300m',
            cpuLimit: '1500m',
            memUsage: '1.2Gi',
            memRequest: '768Mi',
            memLimit: '3Gi'
          },
          containers: [
            { name: 'main-app', image: 'enterprise/api:v2.1.0', ready: true, restartCount: 0 },
            { name: 'istio-proxy', image: 'istio/proxyv2:1.15.0', ready: true, restartCount: 0 }
          ],
          image: 'v2.1.0'
        },
        {
          name: `${namespace}-db`,
          namespace,
          type: 'StatefulSet',
          replicas: 2,
          readyReplicas: 1,
          podNames: [`${namespace}-db-0`, `${namespace}-db-1`],
          labels: { 
            'app': `${namespace}-db`, 
            'tier': 'database',
            'app.logging.k8s.io/group': namespace.includes('payment') ? 'Core Banking' : 'Database Tier',
            'app.logging.k8s.io/environment': 'staging'
          },
          annotations: { 'statefulset.kubernetes.io/pod-name': `${namespace}-db-0` },
          env: { 'DB_PASSWORD': '********', 'DB_USER': 'admin' },
          envSecrets: ['DB_PASSWORD'],
          volumes: [{ name: 'db-data', mountPath: '/var/lib/mysql', readOnly: false }],
          secrets: ['db-root-password'],
          configMaps: ['db-tuning-params'],
          resources: {
            cpuUsage: '120m',
            cpuRequest: '500m',
            cpuLimit: '1000m',
            memUsage: '2.1Gi',
            memRequest: '2Gi',
            memLimit: '4Gi'
          },
          containers: [
            { name: 'db', image: 'enterprise/db:v8.0.31', ready: true, restartCount: 0 }
          ],
          image: 'v8.0.31'
        }
      ];
      resolve(apps);
    }, 400);
  });
};

export const getAppByName = async (
  namespace: string,
  name: string,
  token?: string | null,
  opts?: { revealSecrets?: boolean }
): Promise<AppResource | null> => {
  if (token) {
    try {
      const url = withRevealSecrets(`${API_BASE}/namespaces/${namespace}/apps/${name}`, opts?.revealSecrets);
      return await fetchJSON<AppResource>(url, token);
    } catch (err) {
      console.warn('Failed to load app from backend', err);
    }
  }
  const apps = await getApps(namespace, token, { light: false });
  return apps.find(a => a.name === name) || null;
};

export const getPodLogs = async (podName: string, count: number = 100, containers: string[] = ['main-app'], multiPodNames?: string[]): Promise<LogEntry[]> => {
  if (!USE_MOCKS) {
    throw new Error('Live log streaming requires authentication');
  }
  return new Promise((resolve) => {
    setTimeout(() => {
      const logs: LogEntry[] = [];
      const targets = multiPodNames || [podName];
      
      targets.forEach(targetPod => {
        containers.forEach(container => {
          const countPerSource = Math.max(1, Math.floor(count / (targets.length * containers.length)));
          for (let i = 0; i < countPerSource; i++) {
            // Simulate older logs for "terminated" pods by giving them a larger offset
            const offset = targetPod.includes('terminated') ? Math.floor(Math.random() * 20) + 10 : Math.floor(Math.random() * 10);
            logs.push(generateMockLog(targetPod, container, offset));
          }
        });
      });
      
      resolve(logs.sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()));
    }, 300);
  });
};
