import { ResourceIdentifier, SavedView, ViewFilters, LogViewPreferences } from '../types';
import { ensureOk } from './http';

export type ThemePreference = 'light' | 'dark';

export interface SessionPayload {
  version?: number;
  updated_at?: string;
  active_resources?: ResourceIdentifier[];
  pinned_resources?: ResourceIdentifier[];
  theme?: ThemePreference;
  sidebar_open?: boolean;
  saved_views?: SavedView[];
  view_filters?: ViewFilters;
  active_view_id?: string | null;
  log_view?: LogViewPreferences;
}

const SESSION_ENDPOINT = '/api/v1/session';

const buildHeaders = (token: string) => ({
  'Content-Type': 'application/json',
  'Authorization': `Bearer ${token}`
});

export const fetchSession = async (token: string): Promise<SessionPayload | null> => {
  const res = await fetch(SESSION_ENDPOINT, {
    method: 'GET',
    headers: buildHeaders(token)
  });
  await ensureOk(res, 'sessionService.fetchSession');
  const data = await res.json();
  return data as SessionPayload;
};

export const saveSession = async (token: string, payload: SessionPayload): Promise<void> => {
  const res = await fetch(SESSION_ENDPOINT, {
    method: 'PUT',
    headers: buildHeaders(token),
    body: JSON.stringify(payload)
  });
  await ensureOk(res, 'sessionService.saveSession');
};

export const clearSession = async (token: string): Promise<void> => {
  const res = await fetch(SESSION_ENDPOINT, {
    method: 'DELETE',
    headers: buildHeaders(token)
  });
  await ensureOk(res, 'sessionService.clearSession');
};
