import { lazy, Suspense } from 'react';
import { Navigate, Route, Routes } from 'react-router-dom';
import { AppLayout } from '@/components/layout/AppLayout';
import { Skeleton } from '@/components/common/Skeleton';
import { useTheme } from '@/hooks/useTheme';

const HomePage = lazy(() => import('@/pages/Home/HomePage').then((m) => ({ default: m.HomePage })));
const DownloaderPage = lazy(() =>
  import('@/pages/Downloader/DownloaderPage').then((m) => ({ default: m.DownloaderPage })),
);
const HistoryPage = lazy(() =>
  import('@/pages/History/HistoryPage').then((m) => ({ default: m.HistoryPage })),
);
const MyMediaPage = lazy(() =>
  import('@/pages/MyMedia/MyMediaPage').then((m) => ({ default: m.MyMediaPage })),
);
const ConvertersPage = lazy(() =>
  import('@/pages/Converters/ConvertersPage').then((m) => ({ default: m.ConvertersPage })),
);
const SettingsPage = lazy(() =>
  import('@/pages/Settings/SettingsPage').then((m) => ({ default: m.SettingsPage })),
);
const NotFoundPage = lazy(() =>
  import('@/pages/NotFound/NotFoundPage').then((m) => ({ default: m.NotFoundPage })),
);

function RouteFallback() {
  return (
    <div className="mx-auto max-w-container space-y-4 px-4 py-12 sm:px-6">
      <Skeleton className="h-9 w-48" />
      <Skeleton className="h-56 w-full" />
    </div>
  );
}

export function AppRouter() {
  useTheme();

  return (
    <AppLayout>
      <Suspense fallback={<RouteFallback />}>
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/downloader" element={<DownloaderPage />} />
          <Route path="/history" element={<HistoryPage />} />
          <Route path="/my-media" element={<MyMediaPage />} />
          <Route path="/converters" element={<ConvertersPage />} />
          <Route path="/settings" element={<SettingsPage />} />

          {/* Login is disabled; keep legacy auth paths redirecting home */}
          <Route path="/login" element={<Navigate to="/" replace />} />
          <Route path="/register" element={<Navigate to="/" replace />} />

          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </Suspense>
    </AppLayout>
  );
}
