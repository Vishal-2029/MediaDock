import { create } from 'zustand';
import type { ThemePreference } from '@/types/api';

const THEME_STORAGE_KEY = 'mediadock_theme';

function getInitialTheme(): ThemePreference {
  if (typeof window === 'undefined') return 'dark';
  const stored = window.localStorage.getItem(THEME_STORAGE_KEY);
  if (stored === 'light' || stored === 'dark' || stored === 'system') {
    return stored;
  }
  return 'dark'; // Dark mode is default per design system
}

export function applyThemeToDOM(theme: ThemePreference): void {
  if (typeof document === 'undefined') return;
  const root = document.documentElement;
  const resolved =
    theme === 'system'
      ? window.matchMedia('(prefers-color-scheme: light)').matches
        ? 'light'
        : 'dark'
      : theme;

  root.classList.remove('light', 'dark');
  root.classList.add(resolved);
}

interface ThemeState {
  theme: ThemePreference;
  setTheme: (theme: ThemePreference) => void;
  toggleTheme: () => void;
}

export const useThemeStore = create<ThemeState>((set, get) => ({
  theme: getInitialTheme(),
  setTheme: (theme) => {
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(THEME_STORAGE_KEY, theme);
    }
    applyThemeToDOM(theme);
    set({ theme });
  },
  toggleTheme: () => {
    const current = get().theme;
    const next: ThemePreference = current === 'light' ? 'dark' : 'light';
    get().setTheme(next);
  },
}));

