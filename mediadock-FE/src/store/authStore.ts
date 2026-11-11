import { create } from 'zustand';
import type { UserProfile } from '@/types/api';

interface AuthState {
  user: UserProfile | null;
  isInitialized: boolean;
  setUser: (user: UserProfile | null) => void;
  setInitialized: (initialized: boolean) => void;
  clearAuth: () => void;
}

// Authentication tokens live in secure HttpOnly cookies managed by mediadock-BE;
// no sensitive tokens are stored in localStorage.
export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isInitialized: false,
  setUser: (user) => set({ user, isInitialized: true }),
  setInitialized: (isInitialized) => set({ isInitialized }),
  clearAuth: () => set({ user: null, isInitialized: true }),
}));

