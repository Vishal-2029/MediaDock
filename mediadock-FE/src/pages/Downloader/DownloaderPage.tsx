import { useEffect, useState } from 'react';
import { Link, useLocation, useSearchParams } from 'react-router-dom';
import { AlertCircle, ArrowRight, Trash2 } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/common/Card';
import { Skeleton } from '@/components/common/Skeleton';
import { Button } from '@/components/common/Button';
import { PageMeta } from '@/components/common/PageMeta';
import { UrlInspectForm, type UrlInspectValues } from '@/components/forms/UrlInspectForm';
import { MediaPreviewCard } from '@/components/media/MediaPreviewCard';
import { ProcessingStatusCard } from '@/components/media/ProcessingStatusCard';
import {
  useCreateDownloadMutation,
  useDeleteDownloadMutation,
  useDownloadJobsList,
  useDownloadStatus,
  useInspectMediaMutation,
} from '@/features/media-download/useMediaDownloader';
import { downloadService } from '@/services/downloadService';
import { APIClientError } from '@/services/apiClient';
import { useToastStore } from '@/store/toastStore';
import { formatDate } from '@/utils/format';
import type { MediaInspectResponse } from '@/types/api';

export function DownloaderPage() {
  const [searchParams] = useSearchParams();
  const location = useLocation();
  const routeState = (location.state as { accessToken?: string } | null) || null;
  const initialUrl = searchParams.get('url') || '';

  const [inspected, setInspected] = useState<MediaInspectResponse | null>(null);
  const [lastToken, setLastToken] = useState<string>(routeState?.accessToken || '');
  const [activeJobId, setActiveJobId] = useState<string | null>(null);
  const [downloadingFileId, setDownloadingFileId] = useState<string | null>(null);

  const pushToast = useToastStore((s) => s.push);
  const inspectMutation = useInspectMediaMutation();
  const createDownload = useCreateDownloadMutation();
  const deleteDownload = useDeleteDownloadMutation();
  const statusQuery = useDownloadStatus(activeJobId);
  const recentJobsQuery = useDownloadJobsList(1, 6);

  const handleInspect = async (values: UrlInspectValues) => {
    setInspected(null);
    setActiveJobId(null);
    setLastToken(values.accessToken || '');

    try {
      const result = await inspectMutation.mutateAsync({
        url: values.url,
        access_token: values.accessToken || undefined,
        owner_attested: values.ownerAttested,
      });
      setInspected(result);
    } catch (err) {
      const msg = err instanceof APIClientError ? err.message : 'Media inspection failed';
      pushToast({ title: 'Inspection notice', description: msg, variant: 'error' });
    }
  };

  useEffect(() => {
    if (initialUrl && !inspected && !inspectMutation.isPending && !inspectMutation.isError) {
      void handleInspect({
        url: initialUrl,
        accessToken: routeState?.accessToken || '',
        ownerAttested: searchParams.get('owner_attested') === 'true',
      });
    }
    // Run once on initial mount when navigated with ?url=
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialUrl]);

  const handleStartDownload = async (format: string, quality: string) => {
    if (!inspected) return;
    try {
      const job = await createDownload.mutateAsync({
        media_id: inspected.media_id,
        requested_format: format,
        requested_quality: quality,
        access_token: lastToken || undefined,
      });
      setActiveJobId(job.id);
      pushToast({
        title: 'Download queued',
        description: `Processing ${format.toUpperCase()} (${quality})`,
        variant: 'info',
      });
    } catch (err) {
      const msg = err instanceof APIClientError ? err.message : 'Could not queue download';
      pushToast({ title: 'Download failed', description: msg, variant: 'error' });
    }
  };

  const handleDownloadFile = async (jobId: string, title?: string) => {
    setDownloadingFileId(jobId);
    try {
      await downloadService.downloadFile(jobId, title || 'mediadock-download');
      pushToast({ title: 'Download started', variant: 'success' });
    } catch (err) {
      const msg = err instanceof APIClientError ? err.message : 'Could not fetch file';
      pushToast({ title: 'File error', description: msg, variant: 'error' });
    } finally {
      setDownloadingFileId(null);
    }
  };

  const inspectError =
    inspectMutation.error instanceof APIClientError ? inspectMutation.error : null;

  return (
    <>
      <PageMeta
        title="Downloader"
        description="Inspect authorized media URLs and queue permitted downloads."
        noIndex
      />

      <div className="mx-auto max-w-container space-y-8 px-4 py-10 sm:px-6">
        <div>
          <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">Media Downloader</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Inspect an authorized URL first to view real metadata, formats, and quality options before downloading.
          </p>
        </div>

        <Card>
          <CardContent className="p-6">
            <UrlInspectForm
              initialUrl={initialUrl}
              onSubmit={handleInspect}
              isLoading={inspectMutation.isPending}
            />
          </CardContent>
        </Card>

        {inspectMutation.isPending && (
          <Card>
            <CardContent className="grid gap-6 p-6 md:grid-cols-[280px_1fr]">
              <Skeleton className="aspect-video w-full" />
              <div className="space-y-4">
                <Skeleton className="h-6 w-1/3" />
                <Skeleton className="h-8 w-3/4" />
                <div className="grid grid-cols-3 gap-3">
                  <Skeleton className="h-14" />
                  <Skeleton className="h-14" />
                  <Skeleton className="h-14" />
                </div>
                <Skeleton className="h-11 w-full" />
              </div>
            </CardContent>
          </Card>
        )}

        {inspectError && (
          <Card className="border-destructive/40 bg-destructive/5">
            <CardContent className="flex items-start gap-3 p-5">
              <AlertCircle className="mt-0.5 h-5 w-5 shrink-0 text-destructive" aria-hidden="true" />
              <div className="space-y-1.5 text-sm">
                <p className="font-semibold text-foreground">{inspectError.message}</p>
                {inspectError.details?.alternative && (
                  <p className="text-muted-foreground">
                    <span className="font-medium text-secondary dark:text-cyanHighlight">
                      Authorized alternative:
                    </span>{' '}
                    {inspectError.details.alternative}
                  </p>
                )}
                <div className="pt-1">
                  <Link
                    to="/converters"
                    className="inline-flex items-center gap-1.5 text-xs font-semibold text-primary hover:underline dark:text-cyanHighlight"
                  >
                    Open Converters Studio (No login required)
                    <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
                  </Link>
                </div>
              </div>
            </CardContent>
          </Card>
        )}

        {inspected && (
          <MediaPreviewCard
            media={inspected}
            onStartDownload={handleStartDownload}
            isSubmitting={createDownload.isPending}
          />
        )}

        {activeJobId && statusQuery.data && (
          <ProcessingStatusCard
            title={inspected?.title || 'Download Job'}
            status={statusQuery.data.status}
            progress={statusQuery.data.progress}
            errorMessage={statusQuery.data.error_message}
            onDownloadFile={() => handleDownloadFile(activeJobId, inspected?.title)}
            isDownloadingFile={downloadingFileId === activeJobId}
          />
        )}

        {/* Recent jobs */}
        {recentJobsQuery.data && recentJobsQuery.data.items.length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle className="text-lg">Recent Download Jobs</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              {recentJobsQuery.data.items.map((job) => (
                <div
                  key={job.id}
                  className="flex flex-col justify-between gap-3 rounded-lg border border-border bg-background-secondary p-4 sm:flex-row sm:items-center"
                >
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium text-foreground">
                      {job.media?.title || `Job ${job.id.slice(0, 8)}`}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {job.requested_format.toUpperCase()} ({job.requested_quality}) •{' '}
                      <span className="capitalize">{job.status}</span> ({job.progress}%) •{' '}
                      {formatDate(job.created_at)}
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    {job.status === 'completed' && (
                      <Button
                        size="sm"
                        variant="secondary"
                        isLoading={downloadingFileId === job.id}
                        onClick={() => handleDownloadFile(job.id, job.media?.title)}
                      >
                        Download
                      </Button>
                    )}
                    <Button
                      size="icon"
                      variant="ghost"
                      aria-label="Delete download job"
                      onClick={() => deleteDownload.mutate(job.id)}
                    >
                      <Trash2 className="h-4 w-4 text-muted-foreground hover:text-destructive" />
                    </Button>
                  </div>
                </div>
              ))}
            </CardContent>
          </Card>
        )}
      </div>
    </>
  );
}

