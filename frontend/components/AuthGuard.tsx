
import React, { useState, useEffect } from 'react';
import { AuthUser } from '../types';
import { MOCK_CONFIG, USE_MOCKS } from '../constants';

interface AuthGuardProps {
  children: React.ReactNode;
  onAuth?: (user: AuthUser) => void;
}

type JwtClaims = {
  sub?: string;
  email?: string;
  preferred_username?: string;
  groups?: string[];
};

const parseJwtClaims = (token: string): JwtClaims | null => {
  try {
    const parts = token.split('.');
    if (parts.length < 2) return null;
    const payload = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    const padded = payload.padEnd(payload.length + (4 - (payload.length % 4)) % 4, '=');
    const json = atob(padded);
    return JSON.parse(json) as JwtClaims;
  } catch (err) {
    console.warn('Failed to parse JWT claims', err);
    return null;
  }
};

const AuthGuard: React.FC<AuthGuardProps> = ({ children, onAuth }) => {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const keycloakUrl = (import.meta as any).env?.VITE_KEYCLOAK_URL || MOCK_CONFIG.keycloakUrl;
  const realm = (import.meta as any).env?.VITE_KEYCLOAK_REALM || MOCK_CONFIG.realm;
  const clientId = (import.meta as any).env?.VITE_KEYCLOAK_CLIENT_ID || MOCK_CONFIG.clientId;
  const redirectUri = (import.meta as any).env?.VITE_KEYCLOAK_REDIRECT_URI || `${window.location.origin}${window.location.pathname}`;

  const buildAuthUrl = (state: string) => {
    const url = new URL(`${keycloakUrl}/realms/${realm}/protocol/openid-connect/auth`);
    url.searchParams.set('response_type', 'code');
    url.searchParams.set('client_id', clientId);
    url.searchParams.set('redirect_uri', redirectUri);
    url.searchParams.set('scope', 'openid profile email');
    url.searchParams.set('state', state);
    return url.toString();
  };

  const setUserFromToken = (token?: string | null) => {
    if (token) {
      localStorage.setItem('kubelens_access_token', token);
    }
    const claims = token ? parseJwtClaims(token) : null;
    const groups = claims?.groups ?? (USE_MOCKS ? ['k8s-logs-access', 'developers'] : []);
    const authUser: AuthUser = {
      username: claims?.preferred_username || claims?.email || claims?.sub || (USE_MOCKS ? 'dev_user' : 'unknown'),
      email: claims?.email || (USE_MOCKS ? 'dev@enterprise.com' : ''),
      groups,
      isAuthenticated: Boolean(token) || USE_MOCKS,
      accessToken: token
    };
    setUser(authUser);
    if (onAuth) onAuth(authUser);
  };

  useEffect(() => {
    const run = async () => {
      setLoading(true);
      setError(null);

      const urlParams = new URLSearchParams(window.location.search);
      const code = urlParams.get('code');
      const state = urlParams.get('state');
      const storedState = sessionStorage.getItem('kubelens_oauth_state');

      if (code) {
        if (!state || !storedState || state !== storedState) {
          setError('Invalid login state. Please try again.');
          setLoading(false);
          return;
        }
        try {
          const res = await fetch('/api/v1/auth/token', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ code, redirect_uri: redirectUri })
          });
          if (!res.ok) {
            const text = await res.text();
            throw new Error(text || `Token exchange failed (${res.status})`);
          }
          const payload = await res.json();
          const token = payload.access_token as string | undefined;
          if (!token) {
            throw new Error('Missing access token in response');
          }
          sessionStorage.removeItem('kubelens_oauth_state');
          window.history.replaceState({}, document.title, window.location.pathname + window.location.hash);
          setUserFromToken(token);
          setLoading(false);
          return;
        } catch (err) {
          const message = err instanceof Error ? err.message : 'Login failed';
          setError(message);
          setLoading(false);
          return;
        }
      }

      const token = (import.meta as any).env?.VITE_KUBELENS_TOKEN || localStorage.getItem('kubelens_access_token');
      if (token || USE_MOCKS) {
        setUserFromToken(token);
        setLoading(false);
        return;
      }

      const newState = Math.random().toString(36).slice(2) + Math.random().toString(36).slice(2);
      sessionStorage.setItem('kubelens_oauth_state', newState);
      window.location.href = buildAuthUrl(newState);
    };

    run();
  }, []);

  if (loading) {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen bg-slate-900 text-white">
        <div className="animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-sky-500 mb-4"></div>
        <p className="text-lg font-medium">Connecting to Keycloak SSO...</p>
      </div>
    );
  }

  if (error || !user || !user.groups.includes('k8s-logs-access')) {
    const missingToken = !user?.accessToken;
    return (
      <div className="flex flex-col items-center justify-center min-h-screen bg-slate-900 text-white p-6 text-center">
        <div className="bg-red-500/20 p-8 rounded-xl border border-red-500/50 max-w-md">
          <h1 className="text-2xl font-bold text-red-400 mb-4">Access Denied</h1>
          <p className="text-slate-300 mb-6">
            {error ? (
              <>{error}</>
            ) : missingToken ? (
              <>No access token was found. Please sign in again to access <strong>KubeLens</strong>.</>
            ) : (
              <>Your account does not have the required permissions to access <strong>KubeLens</strong>. 
              You must belong to the <code>k8s-logs-access</code> Keycloak group.</>
            )}
          </p>
          <button 
            onClick={() => {
              if (USE_MOCKS) {
                window.location.reload();
                return;
              }
              const newState = Math.random().toString(36).slice(2) + Math.random().toString(36).slice(2);
              sessionStorage.setItem('kubelens_oauth_state', newState);
              window.location.href = buildAuthUrl(newState);
            }}
            className="px-6 py-2 bg-slate-700 hover:bg-slate-600 rounded-lg transition-colors font-medium"
          >
            Retry Login
          </button>
        </div>
      </div>
    );
  }

  return <>{children}</>;
};

export default AuthGuard;
