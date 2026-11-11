import { useMemo, useState } from 'react';
import { History as HistoryIcon, Search, Trash2, Download } from 'lucide-react';
import { Card, CardContent } from '@/components/common/Card';
import { Button } from '@/components/common/Button';
import { Input, Select } from '@/components/common/Input';
import { Badge } from '@/components/common/Badge';
import { Dialog } from '@/components/common/Dialog';
import { EmptyState } from '@/components/common/EmptyState';
import { Skeleton } from '@/components/common/Skeleton';
import { PageMeta } from '@/components/common/PageMeta';
import { useDeleteHistoryMutation, useHistoryList } from '@/features/media-history/useHistory';
import { downloadService } from '@/services/downloadService';
import { useDebounce } from '@/hooks/useDebounce';
import { formatDate } from '@/utils/format';
import { useToastStore } from '@/store/toastStore';
import { APIClientError } from '@/services/apiClient';
import type { DownloadHistoryRecord } from '@/types/api';

export function HistoryPage() {
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const [formatFilter, setFormatFilter] = useState('all');
  const [dateFilter, setDateFilter] = useState('all');
  const [deleteTarget, setDeleteTarget] = useState<DownloadHistoryRecord | null>(null);

  const debouncedSearch = useDebounce(search, 250);
  const historyQuery = useHistoryList(page, 20);
  const deleteMutation = useDeleteHistoryMutation();
  const pushToast = useToastStore((s) => s.push);

  const filteredItems = useMemo(() => {
    const items = historyQuery.data?.items ?? [];
    const now = Date.now();
    return items.filter((rec) => {
      const matchesSearch =
        !debouncedSearch ||
        rec.file_name.toLowerCase().includes(debouncedSearch.toLowerCase()) ||
        rec.job_id.toLowerCase().includes(debouncedSearch.toLowerCase());

      const ext = rec.file_name.split('.').pop()?.toLowerCase() || '';
      const matchesFormat = formatFilter === 'all' || ext === formatFilter;

      let matchesDate = true;
      if (dateFilter !== 'all') {
        const ts = new Date(rec.downloaded_at).getTime();
        const days = dateFilter === '7d' ? 7 : 30;
        matchesDate = now - ts <= days * 24 * 60 * 60 * 1000;
      }

      return matchesSearch && matchesFormat && matchesDate;
    });
  }, [historyQuery.data?.items, debouncedSearch, formatFilter, dateFilter]);

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    try {
      await deleteMutation.mutateAsync(deleteTarget.id);
      pushToast({ title: 'History record deleted', variant: 'success' });
      setDeleteTarget(null);
    } catch (err) {
      const msg = err instanceof APIClientError ? err.message : 'Could not delete record';
      pushToast({ title: 'Delete failed', description: msg, variant: 'error' });
    }
  };

  const handleRedownload = async (rec: DownloadHistoryRecord) => {
    try {
      await downloadService.downloadFile(rec.job_id, rec.file_name);
    } catch (err) {
      const msg = err instanceof APIClientError ? err.message : 'File may have expired';
      pushToast({ title: 'Download unavailable', description: msg, variant: 'warning' });
    }
  };

  const meta = historyQuery.data?.meta;

  return (
    <>
      <PageMeta title="Download History" description="Review and manage your MediaDock download history." noIndex />

      <div className="mx-auto max-w-container space-y-6 px-4 py-10 sm:px-6">
        <div>
          <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">Download History</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Search, filter, re-download active artifacts, or remove historical entries.
          </p>
        </div>

        {/* Filters */}
        <Card>
          <CardContent className="grid gap-3 p-4 sm:grid-cols-3">
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Search file name or job ID..."
                aria-label="Search history"
                className="pl-9"
              />
            </div>
            <Select
              aria-label="Filter by format"
              value={formatFilter}
              onChange={(e) => setFormatFilter(e.target.value)}
            >
              <option value="all">All Formats</option>
              <option value="mp4">MP4</option>
              <option value="mp3">MP3</option>
              <option value="aac">AAC</option>
              <option value="wav">WAV</option>
              <option value="jpg">JPG</option>
              <option value="png">PNG</option>
            </Select>
            <Select
              aria-label="Filter by date"
              value={dateFilter}
              onChange={(e) => setDateFilter(e.target.value)}
            >
              <option value="all">All Dates</option>
              <option value="7d">Last 7 Days</option>
              <option value="30d">Last 30 Days</option>
            </Select>
          </CardContent>
        </Card>

        {historyQuery.isLoading ? (
          <div className="space-y-3">
            <Skeleton className="h-20 w-full" />
            <Skeleton className="h-20 w-full" />
            <Skeleton className="h-20 w-full" />
          </div>
        ) : filteredItems.length === 0 ? (
          <EmptyState
            icon={HistoryIcon}
            title="No download history found"
            description="Completed downloads appear here automatically once you download an authorized media file."
          />
        ) : (
          <div className="space-y-3">
            {filteredItems.map((rec) => {
              const ext = rec.file_name.split('.').pop()?.toUpperCase() || 'FILE';
              return (
                <Card key={rec.id}>
                  <CardContent className="flex flex-col justify-between gap-4 p-4 sm:flex-row sm:items-center">
                    <div className="min-w-0 space-y-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <p className="truncate text-sm font-semibold text-foreground">{rec.file_name}</p>
                        <Badge variant="secondary">{ext}</Badge>
                        <Badge variant="success">Completed</Badge>
                      </div>
                      <p className="text-xs text-muted-foreground">
                        Downloaded {formatDate(rec.downloaded_at)} • Job {rec.job_id.slice(0, 8)}
                      </p>
                    </div>

                    <div className="flex items-center gap-2">
                      <Button size="sm" variant="outline" onClick={() => handleRedownload(rec)}>
                        <Download className="h-3.5 w-3.5" />
                        Download
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        aria-label={`Delete ${rec.file_name}`}
                        onClick={() => setDeleteTarget(rec)}
                      >
                        <Trash2 className="h-4 w-4 text-muted-foreground hover:text-destructive" />
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              );
            })}
          </div>
        )}

        {meta && meta.total_pages > 1 && (
          <div className="flex items-center justify-between pt-2">
            <p className="text-xs text-muted-foreground">
              Page {meta.page} of {meta.total_pages} ({meta.total} total)
            </p>
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
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

      <Dialog
        open={Boolean(deleteTarget)}
        onClose={() => setDeleteTarget(null)}
        title="Delete history entry?"
        description={`Remove "${deleteTarget?.file_name}" from your download history?`}
        footer={
          <>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>
              Cancel
            </Button>
            <Button variant="destructive" isLoading={deleteMutation.isPending} onClick={confirmDelete}>
              Delete
            </Button>
          </>
        }
      />
    </>
  );
}

