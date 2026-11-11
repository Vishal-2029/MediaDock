import { useEffect, useState } from 'react';
import { Dialog } from '@/components/common/Dialog';
import { Button } from '@/components/common/Button';
import { PlatformBadge } from '@/components/media/PlatformBadge';
import { downloadService } from '@/services/downloadService';
import { formatBytes, formatDuration } from '@/utils/format';
import type { MediaItemRecord } from '@/types/api';

interface MediaPreviewModalProps {
  item: MediaItemRecord | null;
  completedJobId?: string | null;
  onClose: () => void;
}

export function MediaPreviewModal({ item, completedJobId, onClose }: MediaPreviewModalProps) {
  const [blobUrl, setBlobUrl] = useState<string | null>(null);

  useEffect(() => {
    if (!item || !completedJobId) {
      setBlobUrl(null);
      return;
    }

    let revoked = false;
    let objectUrl: string | null = null;

    fetch(downloadService.getFileUrl(completedJobId), { credentials: 'include' })
      .then((res) => {
        if (!res.ok) throw new Error('Preview stream unavailable');
        const ct = res.headers.get('content-type') || '';
        if (ct.includes('application/json')) return null;
        return res.blob();
      })
      .then((blob) => {
        if (!blob || revoked) return;
        objectUrl = URL.createObjectURL(blob);
        setBlobUrl(objectUrl);
      })
      .catch(() => {
        setBlobUrl(null);
      });

    return () => {
      revoked = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [item, completedJobId]);

  if (!item) return null;

  const fmt = item.original_format.toLowerCase();
  const isAudio = ['mp3', 'aac', 'm4a', 'wav', 'flac', 'ogg'].includes(fmt);
  const isImage = ['jpg', 'jpeg', 'png', 'webp'].includes(fmt);

  return (
    <Dialog
      open={Boolean(item)}
      onClose={onClose}
      title={item.title}
      description={`Format: ${item.original_format.toUpperCase()} • Size: ${formatBytes(item.file_size)} • Duration: ${formatDuration(item.duration)}`}
      footer={
        <Button variant="outline" onClick={onClose}>
          Close
        </Button>
      }
    >
      <div className="space-y-4">
        {item.source?.provider && <PlatformBadge provider={item.source.provider} />}

        <div className="overflow-hidden rounded-lg border border-border bg-background-secondary">
          {blobUrl && !isAudio && !isImage ? (
            <video src={blobUrl} controls className="aspect-video w-full" />
          ) : blobUrl && isAudio ? (
            <div className="p-6">
              <audio src={blobUrl} controls className="w-full" />
            </div>
          ) : (
            <img
              src={blobUrl || item.thumbnail_url || '/images/media-placeholder.svg'}
              alt={item.title}
              className="aspect-video w-full object-cover"
              onError={(e) => {
                e.currentTarget.src = '/images/media-placeholder.svg';
              }}
            />
          )}
        </div>
      </div>
    </Dialog>
  );
}

