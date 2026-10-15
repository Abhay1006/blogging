import type { User } from '../types';

const TOKEN_KEY = 'token';
const USER_KEY = 'user';

export interface Session {
  token: string;
  user: User;
}

// Reads the `exp` claim without verifying the signature; the server still
// verifies every token. This only avoids showing an expired session as logged in.
const isExpired = (token: string): boolean => {
  try {
    const payload = token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/');
    const { exp } = JSON.parse(atob(payload));
    return typeof exp !== 'number' || exp * 1000 <= Date.now();
  } catch {
    return true;
  }
};

export const clearSession = () => {
  try {
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(USER_KEY);
  } catch {
    // Storage unavailable (e.g. blocked by the browser); nothing to clear.
  }
};

export const loadSession = (): Session | null => {
  try {
    const token = localStorage.getItem(TOKEN_KEY);
    const userJson = localStorage.getItem(USER_KEY);
    if (!token || !userJson || isExpired(token)) {
      clearSession();
      return null;
    }
    return { token, user: JSON.parse(userJson) as User };
  } catch {
    clearSession();
    return null;
  }
};

export const saveSession = ({ token, user }: Session) => {
  try {
    localStorage.setItem(TOKEN_KEY, token);
    localStorage.setItem(USER_KEY, JSON.stringify(user));
  } catch {
    // Storage unavailable; the session lasts until the page is reloaded.
  }
};

export const getToken = (): string | null => {
  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
};
