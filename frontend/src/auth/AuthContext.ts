import { createContext, useContext } from 'react';
import type { User } from '../types';
import type { Session } from './storage';

export interface AuthContextValue {
  user: User | null;
  isAdmin: boolean;
  login: (session: Session) => void;
  logout: () => void;
}

export const AuthContext = createContext<AuthContextValue | null>(null);

export const useAuth = (): AuthContextValue => {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
};
