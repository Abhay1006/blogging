import { useCallback, useState, type ReactNode } from 'react';
import { Alert, Snackbar, type AlertColor } from '@mui/material';
import { NotificationContext } from './NotificationContext';

interface Notification {
  key: number;
  message: string;
  severity: AlertColor;
}

const NotificationProvider = ({ children }: { children: ReactNode }) => {
  const [notification, setNotification] = useState<Notification | null>(null);

  const notify = useCallback((message: string, severity: AlertColor = 'info') => {
    setNotification({ key: Date.now(), message, severity });
  }, []);

  const handleClose = (_?: unknown, reason?: string) => {
    if (reason !== 'clickaway') setNotification(null);
  };

  return (
    <NotificationContext.Provider value={notify}>
      {children}
      <Snackbar
        key={notification?.key}
        open={notification !== null}
        autoHideDuration={5000}
        onClose={handleClose}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert onClose={handleClose} severity={notification?.severity ?? 'info'} variant="filled" sx={{ width: '100%' }}>
          {notification?.message}
        </Alert>
      </Snackbar>
    </NotificationContext.Provider>
  );
};

export default NotificationProvider;
