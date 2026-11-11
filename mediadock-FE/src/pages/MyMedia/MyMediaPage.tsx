import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { Eye, FolderOpen, Search, Trash2, Download } from 'lucide-react';
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from '@/components/common/Card';
import { Button } from '@/components/common/Button';
import { Input, Select } from '@/components/common/Input';
import { Dialog } from '@/components/common/Dialog';
import { EmptyState } from '@/components/common/EmptyState';
import { Skeleton } from '@/components/common/Skeleton';
import { PageMeta } from '@/components/common/PageMeta';
import { PlatformBadge } from '@/components/media/PlatformBadge';
import { MediaPreviewModal } from '@/features/media-preview/MediaPreviewModal';
import {
  useDeleteMediaMutation,
  useDownloadJobsList,
  useMediaLibrary,
} from '@/features/media-download/useMediaDownloader';
import { downloadService } from '@/services/downloadService';
import { useDebounce } from '@/hooks/useDebounce';
import { formatBytes, formatDate, formatDuration } from '@/utils/format';
import { useToastStore } from '@/store/toastStore';
import { APIClientError } from '@/services/apiClient';
import type { MediaItemRecord } from '@/types/api';

export function MyMediaPage() {
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const [platformFilter, setPlatformFilter] = useState('all');
  const [previewItem, setPreviewItem] = useState<MediaItemRecord | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<MediaItemRecord | null>(null);

  const debouncedSearch = useDebounce(search, 250);
  const mediaQuery = useMediaLibrary(page, 12);
  const jobsQuery = useDownloadJobsList(1, 50);
  const deleteMedia = useDeleteMediaMutation();
  const pushToast = useToastStore((s) => s.push);

  const completedJobByMediaId = useMemo(() => {
    const map = new Map<string, string>();
    for (const job of jobsQuery.data?.items ?? []) {
      if (job.status === 'completed' && !map.has(job.media_id)) {
        map.set(job.media_id, job.id);
      }
    }
    return map;
  }, [jobsQuery.data?.items]);

  const filtered = useMemo(() => {
    const items = mediaQuery.data?.items ?? [];
    return items.filter((item) => {
      const matchesSearch =
        !debouncedSearch || item.title.toLowerCase().includes(debouncedSearch.toLowerCase());
      const provider = item.source?.provider || 'generic';
      const matchesPlatform = platformFilter === 'all' || provider === platformFilter;
      return matchesSearch && matchesPlatform;
    });
  }, [mediaQuery.data?.items, debouncedSearch, platformFilter]);

  const handleConfirmDelete = async () => {
    if (!deleteTarget) return;
    try {
      await deleteMedia.mutateAsync(deleteTarget.id);
      pushToast({ title: 'Media removed from library', variant: 'success' });
      setDeleteTarget(null);
    } catch (err) {
      const msg = err instanceof APIClientError ? err.message : 'Could not delete media item';
      pushToast({ title: 'Delete failed', description: msg, variant: 'error' });
    }
  };

  const handleDownload = async (mediaId: string, title: string) => {
    const jobId = completedJobByMediaId.get(mediaId);
    if (!jobId) return;
    try {
      await downloadService.downloadFile(jobId, title);
    } catch (err) {
      const msg = err instanceof APIClientError ? err.message : 'File is no longer available';
      pushToast({ title: 'Download unavailable', description: msg, variant: 'warning' });
    }
  };

  const meta = mediaQuery.data?.meta;

  return (
    <>
      <PageMeta title="My Media" description="Browse, preview, and manage your saved authorized media." noIndex />

      <div className="mx-auto max-w-container space-y-6 px-4 py-10 sm:px-6">
        <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
          <div>
            <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">My Media</h1>
            <p className="mt-1 text-sm text-muted-foreground">
              Your inspected and saved authorized media library.
            </p>
          </div>
          <Link to="/downloader">
            <Button variant="primary">Inspect New Media</Button>
          </Link>
        </div>

        <Card>
          <CardContent className="grid gap-3 p-4 sm:grid-cols-2">
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Search saved media..."
                aria-label="Search saved media"
                className="pl-9"
              />
            </div>
            <Select
              aria-label="Filter by platform"
              value={platformFilter}
              onChange={(e) => setPlatformFilter(e.target.value)}
            >
              <option value="all">All Platforms</option>
              <option value="instagram">Instagram</option>
              <option value="generic">Direct Authorized Media</option>
            </Select>
          </CardContent>
        </Card>

        {mediaQuery.isLoading ? (
          <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            <Skeleton className="h-72" />
            <Skeleton className="h-72" />
            <Skeleton className="h-72" />
          </div>
        ) : filtered.length === 0 ? (
          <EmptyState
            icon={FolderOpen}
            title="Your media library is empty"
            description="Inspect an authorized media URL in the Downloader to save items to your library."
            action={
              <Link to="/downloader">
                <Button variant="primary">Open Downloader</Button>
              </Link>
            }
          />
        ) : (
          <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            {filtered.map((item) => {
              const completedJobId = completedJobByMediaId.get(item.id);
              return (
                <Card key={item.id} className="flex flex-col overflow-hidden">
                  <div className="relative aspect-video bg-background-secondary">
                    <img
                      src={item.thumbnail_url || '/images/media-placeholder.svg'}
                      alt={item.title}
                      className="h-full w-full object-cover"
                      onError={(e) => {
                        e.currentTarget.src = '/images/media-placeholder.svg';
                      }}
                    />
                  </div>
                  <CardHeader className="pb-2">
                    <div className="flex items-center justify-between gap-2">
                      {item.source?.provider && <PlatformBadge provider={item.source.provider} />}
                      <span className="text-xs font-medium uppercase text-muted-foreground">
                        {item.original_format}
                      </span>
                    </div>
                    <CardTitle className="mt-1 line-clamp-2 text-base">{item.title}</CardTitle>
                  </CardHeader>
                  <CardContent className="flex-1 pt-0 text-xs text-muted-foreground">
                    <p>
                      Size: {formatBytes(item.file_size)} • Duration: {formatDuration(item.duration)}
                    </p>
                    <p className="mt-1">Added {formatDate(item.created_at)}</p>
                  </CardContent>
                  <CardFooter className="justify-between border-t border-border pt-3">
                    <div className="flex gap-2">
                      <Button size="sm" variant="outline" onClick={() => setPreviewItem(item)}>
                        <Eye className="h-3.5 w-3.5" />
                        Preview
                      </Button>
                      {completedJobId && (
                        <Button
                          size="sm"
                          variant="secondary"
                          onClick={() => handleDownload(item.id, item.title)}
                        >
                          <Download className="h-3.5 w-3.5" />
                          Download
                        </Button>
                      )}
                    </div>
                    <Button
                      size="icon"
                      variant="ghost"
                      aria-label={`Delete ${item.title}`}
                      onClick={() => setDeleteTarget(item)}
                    >
                      <Trash2 className="h-4 w-4 text-muted-foreground hover:text-destructive" />
                    </Button>
                  </CardFooter>
                </Card>
              );
            })}
          </div>
        )}

        {meta && meta.total_pages > 1 && (
          <div className="flex items-center justify-between pt-2">
            <p className="text-xs text-muted-foreground">
              Page {meta.page} of {meta.total_pages}
            </p>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
                Previous
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={page >= meta.total_pages}
                onClick={() => setPage((p) => p + 1)}
              >
                Next
              </Button>
            </div>
          </div>
        )}
      </div>

      <MediaPreviewModal
        item={previewItem}
        completedJobId={previewItem ? completedJobByMediaId.get(previewItem.id) : null}
        onClose={() => setPreviewItem(null)}
      />

      <Dialog
        open={Boolean(deleteTarget)}
        onClose={() => setDeleteTarget(null)}
        title="Delete media item?"
        description={`Remove "${deleteTarget?.title}" from your saved library?`}
        footer={
          <>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>
              Cancel
            </Button>
            <Button variant="destructive" isLoading={deleteMedia.isPending} onClick={handleConfirmDelete}>
              Delete
            </Button>
          </>
        }
      />
    </>
  );
}

