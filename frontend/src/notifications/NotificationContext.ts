import { createContext, useContext } from 'react';
import type { AlertColor } from '@mui/material';
import { getErrorMessage, isSessionExpired } from '../api/client';

export type Notify = (message: string, severity?: AlertColor) => void;

export const NotificationContext = createContext<Notify | null>(null);

export const useNotify = (): Notify => {
  const notify = useContext(NotificationContext);
  if (!notify) throw new Error('useNotify must be used within NotificationProvider');
  return notify;
};

/**
 * Returns a callback that shows a request error. Expired sessions are skipped
 * because the auth provider already reports them.
 */
export const useNotifyError = () => {
  const notify = useNotify();
  return (error: unknown, fallback: string) => {
    if (!isSessionExpired(error)) notify(getErrorMessage(error, fallback), 'error');
  };
};
