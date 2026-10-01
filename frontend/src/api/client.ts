import axios, { isAxiosError } from 'axios';
import { getToken } from '../auth/storage';

const DEFAULT_API_URL = 'http://localhost:10000';
const API_URL = (import.meta.env.VITE_API_URL || DEFAULT_API_URL).replace(/\/+$/, '');

if (!import.meta.env.VITE_API_URL && import.meta.env.PROD) {
  console.error(`VITE_API_URL is not set; falling back to ${DEFAULT_API_URL}.`);
}

export const api = axios.create({ baseURL: API_URL });

api.interceptors.request.use((config) => {
  const token = getToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

let unauthorizedHandler: (() => void) | null = null;

/** Registers the callback run when an authenticated request is rejected with 401. */
export const setUnauthorizedHandler = (handler: (() => void) | null) => {
  unauthorizedHandler = handler;
};

/**
 * True when an authenticated request was rejected because the session expired.
 * A 401 without a token (e.g. from /login) is just bad credentials.
 */
export const isSessionExpired = (error: unknown): boolean =>
  isAxiosError(error) && error.response?.status === 401 && Boolean(error.config?.headers?.Authorization);

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (isSessionExpired(error)) {
      unauthorizedHandler?.();
    }
    return Promise.reject(error);
  },
);

export const getErrorMessage = (error: unknown, fallback: string): string => {
  if (isAxiosError<{ error?: string }>(error)) {
    return error.response?.data?.error ?? fallback;
  }
  return fallback;
};

export const isNotFound = (error: unknown): boolean =>
  isAxiosError(error) && (error.response?.status === 404 || error.response?.status === 400);
