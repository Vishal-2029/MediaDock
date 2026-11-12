import axios, { AxiosError, type AxiosRequestConfig } from 'axios';
import type { APIResponse } from '@/types/api';

export const API_BASE_URL =
  import.meta.env.VITE_API_BASE_URL?.replace(/\/+$/, '') || '/api/v1';

export class APIClientError extends Error {
  public readonly status: number;
  public readonly code: string;
  public readonly details?: Record<string, string>;

  constructor(message: string, status = 500, code = 'UNKNOWN_ERROR', details?: Record<string, string>) {
    super(message);
    this.name = 'APIClientError';
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  withCredentials: true,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

let refreshPromise: Promise<void> | null = null;

apiClient.interceptors.response.use(
  (response) => response,
  async (error: AxiosError<APIResponse<unknown>>) => {
    const originalRequest = error.config as (AxiosRequestConfig & { _retry?: boolean }) | undefined;
    const status = error.response?.status ?? 0;

    if (
      status === 401 &&
      originalRequest &&
      !originalRequest._retry &&
      !originalRequest.url?.includes('/auth/login') &&
      !originalRequest.url?.includes('/auth/register') &&
      !originalRequest.url?.includes('/auth/refresh')
    ) {
      originalRequest._retry = true;
      try {
        if (!refreshPromise) {
          refreshPromise = apiClient.post('/auth/refresh', {}).then(() => undefined);
        }
        await refreshPromise;
        refreshPromise = null;
        return apiClient(originalRequest);
      } catch {
        refreshPromise = null;
      }
    }

    const apiErr = error.response?.data?.error;
    if (apiErr) {
      return Promise.reject(
        new APIClientError(apiErr.message, status || 500, apiErr.code, apiErr.details),
      );
    }

    if (axios.isCancel(error)) {
      return Promise.reject(new APIClientError('Request cancelled', 0, 'REQUEST_CANCELLED'));
    }

    const fallbackMessage =
      status === 0
        ? 'Cannot reach the MediaDock API server. Make sure the backend (:8080) or Docker stack is running.'
        : error.message || 'Network error while communicating with MediaDock API';

    return Promise.reject(
      new APIClientError(
        fallbackMessage,
        status || 0,
        'NETWORK_ERROR',
      ),
    );
  },
);

export function extractData<T>(response: { data: APIResponse<T> }): T {
  if (!response.data.success || response.data.data === undefined) {
    const err = response.data.error;
    throw new APIClientError(err?.message || 'Unexpected API response', 500, err?.code || 'API_ERROR', err?.details);
  }
  return response.data.data;
}

