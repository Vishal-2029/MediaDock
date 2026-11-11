import { Navigate, Outlet, useLocation } from 'react-router-dom';
import { Skeleton } from '@/components/common/Skeleton';
import { useAuthStore } from '@/store/authStore';

export function ProtectedRoute() {
  const user = useAuthStore((s) => s.user);
  const isInitialized = useAuthStore((s) => s.isInitialized);
  const location = useLocation();

  if (!isInitialized) {
    return (
      <div className="mx-auto max-w-container space-y-4 px-4 py-12 sm:px-6">
        <Skeleton className="h-10 w-56" />
        <Skeleton className="h-48 w-full" />
      </div>
    );
  }

  if (!user) {
    const target = `${location.pathname}${location.search}`;
    return <Navigate to={`/login?redirect=${encodeURIComponent(target)}`} replace />;
  }

  return <Outlet />;
}

