import { Link } from 'react-router-dom';
import { Home } from 'lucide-react';
import { Button } from '@/components/common/Button';
import { PageMeta } from '@/components/common/PageMeta';

export function NotFoundPage() {
  return (
    <>
      <PageMeta title="Page Not Found" description="The requested page could not be found." noIndex />
      <div className="mx-auto flex max-w-container flex-col items-center justify-center px-4 py-24 text-center sm:px-6">
        <p className="text-sm font-semibold uppercase tracking-widest text-secondary dark:text-cyanHighlight">404</p>
        <h1 className="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">Page not found</h1>
        <p className="mt-3 max-w-md text-sm text-muted-foreground">
          The page you are looking for does not exist or has moved.
        </p>
        <Link to="/" className="mt-6">
          <Button variant="primary">
            <Home className="h-4 w-4" />
            Back to Home
          </Button>
        </Link>
      </div>
    </>
  );
}

