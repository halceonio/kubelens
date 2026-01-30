
import React from 'react';

export const COLORS = {
  bg: '#0f172a',
  sidebar: '#1e293b',
  card: '#334155',
  accent: '#38bdf8',
  error: '#ef4444',
  warning: '#f59e0b',
  info: '#10b981',
};

export const MOCK_CONFIG = {
  keycloakUrl: 'https://sso.enterprise.com',
  realm: 'production',
  clientId: 'kubelens-client',
  allowedNamespaces: ['ai-system', 'apps', 'api', 'db', 'internal-apps'],
  podFilterRegex: '.*',
  label_prefix: 'app.sgz.ai',
  appGroups: {
    enabled: true,
    labels: {
      selector: "app.sgz.ai/group",
      name: "app.sgz.ai/displayname",
      environment: "app.sgz.ai/env",
      version: "app.sgz.ai/version"
    }
  }
};

export const MOCK_NAMESPACES = [
  { name: 'payment-svc' },
  { name: 'inventory-svc' },
  { name: 'auth-svc' },
  { name: 'default' }
];

export const MOCK_PODS: Record<string, any[]> = {
  'payment-svc': [
    { 
      name: 'payment-api-7f8d6c', 
      status: 'Running', 
      restarts: 0, 
      age: '4d', 
      namespace: 'payment-svc',
      labels: {
        'logger.app.k8s.io/ingress': 'api.payments.enterprise.com',
        'logger.app.k8s.io/owner': 'fintech-team',
        'app': 'payment-api'
      }
    },
    { 
      name: 'payment-worker-9b2c3d', 
      status: 'Running', 
      restarts: 2, 
      age: '12h', 
      namespace: 'payment-svc',
      labels: {
        'logger.app.k8s.io/queue': 'payment-tasks-v1',
        'app': 'payment-worker'
      }
    },
  ],
  'inventory-svc': [
    { 
      name: 'stock-manager-5v4e1r', 
      status: 'Failed', 
      restarts: 15, 
      age: '1d', 
      namespace: 'inventory-svc',
      labels: {
        'logger.app.k8s.io/alert-channel': '#inventory-critical',
        'app': 'stock-manager'
      }
    },
    { name: 'catalog-api-1x2y3z', status: 'Running', restarts: 0, age: '10d', namespace: 'inventory-svc' },
  ],
  'auth-svc': [
    { name: 'keycloak-proxy-8h9j0k', status: 'Running', restarts: 1, age: '30d', namespace: 'auth-svc' },
  ]
};
