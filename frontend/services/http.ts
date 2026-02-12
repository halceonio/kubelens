import { emitUnauthorized } from './authEvents';

export class ApiError extends Error {
  status: number;

  constructor(status: number, message?: string) {
    super(message || `Request failed: ${status}`);
    this.name = 'ApiError';
    this.status = status;
  }
}

export const isApiErrorStatus = (error: unknown, status: number) => {
  return error instanceof ApiError && error.status === status;
};

export const ensureOk = async (res: Response, source?: string): Promise<Response> => {
  if (res.ok) return res;

  if (res.status === 401) {
    emitUnauthorized({ source, status: res.status });
  }

  let bodyText = '';
  try {
    bodyText = await res.text();
  } catch {
    // no-op
  }

  throw new ApiError(res.status, bodyText || `Request failed: ${res.status}`);
};
