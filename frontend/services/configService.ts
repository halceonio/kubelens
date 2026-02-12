import { UiConfig } from '../types';
import { ensureOk } from './http';

const API_BASE = '/api/v1';

const buildHeaders = (token?: string | null) => {
  if (!token) return {};
  return { Authorization: `Bearer ${token}` };
};

export const fetchConfig = async (token: string): Promise<UiConfig> => {
  const res = await fetch(`${API_BASE}/config`, { headers: buildHeaders(token) });
  await ensureOk(res, 'configService');
  return res.json();
};
