import { useEffect, useMemo, useState } from 'react';
import { Download, ShieldCheck, Info, Clock, HardDrive, Film } from 'lucide-react';
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from '@/components/common/Card';
import { Button } from '@/components/common/Button';
import { Label, Select } from '@/components/common/Input';
import { PlatformBadge } from '@/components/media/PlatformBadge';
import { formatBytes, formatDuration } from '@/utils/format';
import type { MediaInspectResponse } from '@/types/api';

interface MediaPreviewCardProps {
  media: MediaInspectResponse;
  onStartDownload: (format: string, quality: string) => void;
  isSubmitting?: boolean;
}

export function MediaPreviewCard({ media, onStartDownload, isSubmitting }: MediaPreviewCardProps) {
  const formats = media.available_formats ?? [];

  const uniqueFormats = useMemo(
    () => Array.from(new Set(formats.map((f) => f.format))),
    [formats],
  );

  const [selectedFormat, setSelectedFormat] = useState<string>(uniqueFormats[0] || media.original_format || '');

  const availableQualities = useMemo(
    () =>
      formats
        .filter((f) => f.format === selectedFormat)
        .map((f) => ({ quality: f.quality, description: f.description })),
    [formats, selectedFormat],
  );

  const [selectedQuality, setSelectedQuality] = useState<string>(availableQualities[0]?.quality || 'original');

  useEffect(() => {
    if (uniqueFormats.length > 0 && !uniqueFormats.includes(selectedFormat)) {
      setSelectedFormat(uniqueFormats[0]);
    }
  }, [uniqueFormats, selectedFormat]);

  useEffect(() => {
    if (availableQualities.length > 0 && !availableQualities.some((q) => q.quality === selectedQuality)) {
      setSelectedQuality(availableQualities[0].quality);
    }
  }, [availableQualities, selectedQuality]);

  return (
    <Card className="overflow-hidden">
      <div className="grid gap-6 md:grid-cols-[280px_1fr]">
        <div className="relative aspect-video bg-background-secondary md:aspect-auto md:min-h-[220px]">
          <img
            src={media.thumbnail_url || '/images/media-placeholder.svg'}
            alt={media.title}
            className="h-full w-full object-cover"
            onError={(e) => {
              e.currentTarget.src = '/images/media-placeholder.svg';
            }}
          />
        </div>

        <div className="flex flex-col justify-between">
          <div>
            <CardHeader className="pb-2">
              <div className="flex flex-wrap items-center gap-2">
                <PlatformBadge provider={media.provider} />
                {media.authorization?.authorized ? (
                  <span className="inline-flex items-center gap-1 rounded-full bg-success/15 px-2.5 py-0.5 text-xs font-medium text-success">
                    <ShieldCheck className="h-3.5 w-3.5" aria-hidden="true" />
                    Authorized ({media.authorization.authorization_method})
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1 rounded-full bg-warning/15 px-2.5 py-0.5 text-xs font-medium text-warning">
                    <Info className="h-3.5 w-3.5" aria-hidden="true" />
                    Preview Only
                  </span>
                )}
              </div>
              <CardTitle className="mt-2 break-words text-xl">{media.title}</CardTitle>
            </CardHeader>

            <CardContent className="space-y-5">
              <dl className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-3">
                <div className="rounded-lg border border-border bg-background-secondary px-3 py-2">
                  <dt className="flex items-center gap-1.5 text-xs text-muted-foreground">
                    <Clock className="h-3.5 w-3.5" /> Duration
                  </dt>
                  <dd className="mt-0.5 font-medium">{formatDuration(media.duration)}</dd>
                </div>
                <div className="rounded-lg border border-border bg-background-secondary px-3 py-2">
                  <dt className="flex items-center gap-1.5 text-xs text-muted-foreground">
                    <HardDrive className="h-3.5 w-3.5" /> File Size
                  </dt>
                  <dd className="mt-0.5 font-medium">{formatBytes(media.file_size)}</dd>
                </div>
                <div className="rounded-lg border border-border bg-background-secondary px-3 py-2">
                  <dt className="flex items-center gap-1.5 text-xs text-muted-foreground">
                    <Film className="h-3.5 w-3.5" /> Source Format
                  </dt>
                  <dd className="mt-0.5 font-medium uppercase">{media.original_format || '—'}</dd>
                </div>
              </dl>

              {media.downloadable !== false && formats.length > 0 ? (
                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="space-y-1.5">
                    <Label htmlFor="format-select">Available Format</Label>
                    <Select
                      id="format-select"
                      value={selectedFormat}
                      onChange={(e) => setSelectedFormat(e.target.value)}
                    >
                      {uniqueFormats.map((fmt) => (
                        <option key={fmt} value={fmt}>
                          {fmt.toUpperCase()}
                        </option>
                      ))}
                    </Select>
                  </div>

                  <div className="space-y-1.5">
                    <Label htmlFor="quality-select">Available Quality</Label>
                    <Select
                      id="quality-select"
                      value={selectedQuality}
                      onChange={(e) => setSelectedQuality(e.target.value)}
                    >
                      {availableQualities.map((q) => (
                        <option key={`${selectedFormat}-${q.quality}`} value={q.quality}>
                          {q.quality} — {q.description}
                        </option>
                      ))}
                    </Select>
                  </div>
                </div>
              ) : (
                <div className="rounded-lg border border-border bg-background-secondary p-3 text-sm text-muted-foreground">
                  <p className="font-medium text-foreground">Preview available — direct download blocked</p>
                  <p className="mt-1">
                    {media.download_blocked_reason ||
                      media.authorization?.reason ||
                      'No downloadable format options were returned for this media item.'}
                  </p>
                  <p className="mt-2">
                    Have your own exported or original video file?{' '}
                    <a href="/converters" className="font-medium text-primary underline underline-offset-4">
                      Open Media Converters
                    </a>
                  </p>
                </div>
              )}
            </CardContent>
          </div>

          <CardFooter className="justify-end border-t border-border pt-4">
            <Button
              variant="primary"
              size="lg"
              disabled={media.downloadable === false || !selectedFormat || formats.length === 0}
              isLoading={isSubmitting}
              onClick={() => onStartDownload(selectedFormat, selectedQuality)}
            >
              <Download className="h-4 w-4" aria-hidden="true" />
              Download Media
            </Button>
          </CardFooter>
        </div>
      </div>
    </Card>
  );
}

