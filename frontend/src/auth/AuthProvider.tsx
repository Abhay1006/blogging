import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { AuthContext } from './AuthContext';
import { clearSession, loadSession, saveSession, type Session } from './storage';
import { setUnauthorizedHandler } from '../api/client';
import { useNotify } from '../notifications/NotificationContext';

const AuthProvider = ({ children }: { children: ReactNode }) => {
  const [session, setSession] = useState<Session | null>(loadSession);
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const notify = useNotify();

  // Cached responses include the viewer's own votes, so drop them when the viewer changes.
  const login = useCallback((next: Session) => {
    saveSession(next);
    setSession(next);
    queryClient.clear();
  }, [queryClient]);

  const logout = useCallback(() => {
    clearSession();
    setSession(null);
    queryClient.clear();
  }, [queryClient]);

  useEffect(() => {
    setUnauthorizedHandler(() => {
      logout();
      notify('Your session has expired. Please log in again.', 'warning');
      navigate('/login');
    });
    return () => setUnauthorizedHandler(null);
  }, [logout, navigate, notify]);

  const value = useMemo(() => ({
    user: session?.user ?? null,
    isAdmin: session?.user.role === 'admin',
    login,
    logout,
  }), [session, login, logout]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};

export default AuthProvider;
