
import React, { useState, useEffect } from 'react';
import { AuthUser } from '../types';
import { USE_MOCKS } from '../constants';

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

  const simulateLogin = () => {
    setLoading(true);
    // Simulating Keycloak redirection and login
    setTimeout(() => {
      const token = (import.meta as any).env?.VITE_KUBELENS_TOKEN || localStorage.getItem('kubelens_access_token') || undefined;
      if (token) {
        localStorage.setItem('kubelens_access_token', token);
      }
      if (!token && !USE_MOCKS) {
        setUser(null);
        setLoading(false);
        return;
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
      setLoading(false);
    }, 1500);
  };

  useEffect(() => {
    // In a real app, this would check the Keycloak session
    simulateLogin();
  }, []);

  if (loading) {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen bg-slate-900 text-white">
        <div className="animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-sky-500 mb-4"></div>
        <p className="text-lg font-medium">Connecting to Keycloak SSO...</p>
      </div>
    );
  }

  if (!user || !user.groups.includes('k8s-logs-access')) {
    const missingToken = !user?.accessToken;
    return (
      <div className="flex flex-col items-center justify-center min-h-screen bg-slate-900 text-white p-6 text-center">
        <div className="bg-red-500/20 p-8 rounded-xl border border-red-500/50 max-w-md">
          <h1 className="text-2xl font-bold text-red-400 mb-4">Access Denied</h1>
          <p className="text-slate-300 mb-6">
            {missingToken ? (
              <>No access token was found. Please sign in again to access <strong>KubeLens</strong>.</>
            ) : (
              <>Your account does not have the required permissions to access <strong>KubeLens</strong>. 
              You must belong to the <code>k8s-logs-access</code> Keycloak group.</>
            )}
          </p>
          <button 
            onClick={() => window.location.reload()}
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
