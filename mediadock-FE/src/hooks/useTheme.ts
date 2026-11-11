import { useEffect } from 'react';
import { applyThemeToDOM, useThemeStore } from '@/store/themeStore';

export function useTheme() {
  const theme = useThemeStore((s) => s.theme);
  const setTheme = useThemeStore((s) => s.setTheme);
  const toggleTheme = useThemeStore((s) => s.toggleTheme);

  useEffect(() => {
    applyThemeToDOM(theme);
    if (theme !== 'system') return;

    const mq = window.matchMedia('(prefers-color-scheme: light)');
    const onChange = () => applyThemeToDOM('system');
    mq.addEventListener('change', onChange);
    return () => mq.removeEventListener('change', onChange);
  }, [theme]);

  return { theme, setTheme, toggleTheme };
}

