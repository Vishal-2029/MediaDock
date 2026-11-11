import { useState } from 'react';
import { Link, NavLink } from 'react-router-dom';
import { Menu, Settings, X } from 'lucide-react';
import { Button } from '@/components/common/Button';
import { ThemeToggle } from '@/components/navigation/ThemeToggle';
import { cn } from '@/utils/cn';

const NAV_LINKS = [
  { label: 'Home', to: '/' },
  { label: 'Downloader', to: '/downloader' },
  { label: 'Converters', to: '/converters' },
  { label: 'My Media', to: '/my-media' },
  { label: 'History', to: '/history' },
];

export function Navbar() {
  const [mobileOpen, setMobileOpen] = useState(false);

  return (
    <header className="sticky top-0 z-40 border-b border-border bg-background/95 backdrop-blur-md">
      <div className="mx-auto flex h-16 max-w-container items-center justify-between gap-4 px-4 sm:px-6">
        <Link
          to="/"
          className="flex shrink-0 items-center gap-2.5 text-foreground focus-visible:rounded-md"
          onClick={() => setMobileOpen(false)}
        >
          <img src="/logo.svg" alt="MediaDock logo" className="h-9 w-9 shrink-0" />
          <div className="flex flex-col leading-tight">
            <span className="whitespace-nowrap text-base font-bold tracking-tight">MediaDock</span>
            <span className="hidden whitespace-nowrap text-[11px] font-medium text-muted-foreground sm:inline">
              Media Workspace
            </span>
          </div>
        </Link>

        {/* Desktop navigation */}
        <nav aria-label="Main navigation" className="hidden items-center gap-1 md:flex">
          {NAV_LINKS.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              className={({ isActive }) =>
                cn(
                  'shrink-0 whitespace-nowrap rounded-lg px-3.5 py-2 text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-primary/10 font-semibold text-primary dark:bg-primary/20 dark:text-cyanHighlight'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground',
                )
              }
            >
              {item.label}
            </NavLink>
          ))}
        </nav>

        {/* Right actions */}
        <div className="flex shrink-0 items-center gap-2">
          <ThemeToggle />

          <NavLink
            to="/settings"
            className={({ isActive }) =>
              cn(
                'hidden shrink-0 items-center gap-1.5 whitespace-nowrap rounded-lg border border-border px-3 py-1.5 text-sm font-medium transition-colors sm:inline-flex',
                isActive
                  ? 'border-primary/40 bg-primary/10 text-primary dark:text-cyanHighlight'
                  : 'bg-card text-foreground hover:bg-muted',
              )
            }
          >
            <Settings className="h-4 w-4" aria-hidden="true" />
            <span>Settings</span>
          </NavLink>

          <Button
            variant="ghost"
            size="icon"
            className="md:hidden"
            onClick={() => setMobileOpen((v) => !v)}
            aria-expanded={mobileOpen}
            aria-label="Toggle mobile menu"
          >
            {mobileOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
          </Button>
        </div>
      </div>

      {/* Mobile navigation drawer */}
      {mobileOpen && (
        <div className="border-t border-border bg-background-secondary px-4 py-4 md:hidden">
          <nav aria-label="Mobile navigation" className="flex flex-col gap-1">
            {NAV_LINKS.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.to === '/'}
                onClick={() => setMobileOpen(false)}
                className={({ isActive }) =>
                  cn(
                    'rounded-lg px-3 py-2.5 text-sm font-medium transition-colors',
                    isActive
                      ? 'bg-primary/10 font-semibold text-primary dark:bg-primary/20 dark:text-cyanHighlight'
                      : 'text-muted-foreground hover:bg-card hover:text-foreground',
                  )
                }
              >
                {item.label}
              </NavLink>
            ))}

            <NavLink
              to="/settings"
              onClick={() => setMobileOpen(false)}
              className={({ isActive }) =>
                cn(
                  'mt-1 flex items-center gap-2 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-primary/10 font-semibold text-primary dark:bg-primary/20 dark:text-cyanHighlight'
                    : 'text-muted-foreground hover:bg-card hover:text-foreground',
                )
              }
            >
              <Settings className="h-4 w-4" aria-hidden="true" />
              Settings
            </NavLink>
          </nav>
        </div>
      )}
    </header>
  );
}
