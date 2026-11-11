import { Moon, Sun } from 'lucide-react';
import { Button } from '@/components/common/Button';
import { useTheme } from '@/hooks/useTheme';
import { useAuthStore } from '@/store/authStore';
import { settingsService } from '@/services/settingsService';

export function ThemeToggle() {
  const { theme, toggleTheme } = useTheme();
  const user = useAuthStore((s) => s.user);
  const isLight = theme === 'light';

  const handleToggle = () => {
    const next = isLight ? 'dark' : 'light';
    toggleTheme();
    if (user) {
      void settingsService.update({ theme: next }).catch(() => undefined);
    }
  };

  return (
    <Button
      variant="ghost"
      size="icon"
      onClick={handleToggle}
      aria-label={isLight ? 'Switch to dark mode' : 'Switch to light mode'}
      title={isLight ? 'Switch to dark mode' : 'Switch to light mode'}
    >
      {isLight ? <Moon className="h-4 w-4" /> : <Sun className="h-4 w-4 text-cyanHighlight" />}
    </Button>
  );
}

