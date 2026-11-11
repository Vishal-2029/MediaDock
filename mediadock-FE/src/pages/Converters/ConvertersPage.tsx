import { useState, type ChangeEvent, type FormEvent } from 'react';
import { FileVideo, UploadCloud, Wand2 } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/common/Card';
import { Button } from '@/components/common/Button';
import { Label, Select } from '@/components/common/Input';
import { Progress } from '@/components/common/Progress';
import { PageMeta } from '@/components/common/PageMeta';
import { ProcessingStatusCard } from '@/components/media/ProcessingStatusCard';
import { CONVERSION_TARGET_FORMATS, RESOLUTION_PRESETS } from '@/constants/platforms';
import { useConversionStatus, useCreateConversionMutation } from '@/features/media-conversion/useConversion';
import { conversionService } from '@/services/conversionService';
import { APIClientError } from '@/services/apiClient';
import { useToastStore } from '@/store/toastStore';
import { formatBytes } from '@/utils/format';

export function ConvertersPage() {
  const [file, setFile] = useState<File | null>(null);
  const [targetFormat, setTargetFormat] = useState<string>('mp4');
  const [targetResolution, setTargetResolution] = useState<string>('original');
  const [compress, setCompress] = useState(false);
  const [extractAudio, setExtractAudio] = useState(false);
  const [uploadPct, setUploadPct] = useState<number>(0);
  const [activeConvId, setActiveConvId] = useState<string | null>(null);
  const [downloading, setDownloading] = useState(false);

  const pushToast = useToastStore((s) => s.push);
  const createConversion = useCreateConversionMutation();
  const statusQuery = useConversionStatus(activeConvId);

  const selectedFmtMeta = CONVERSION_TARGET_FORMATS.find((f) => f.value === targetFormat);
  const isAudioTarget = selectedFmtMeta?.kind === 'audio';

  const handleFileChange = (e: ChangeEvent<HTMLInputElement>) => {
    const picked = e.target.files?.[0] || null;
    setFile(picked);
  };

  const handleFormatChange = (nextFmt: string) => {
    setTargetFormat(nextFmt);
    const meta = CONVERSION_TARGET_FORMATS.find((f) => f.value === nextFmt);
    if (meta?.kind === 'audio') {
      setExtractAudio(true);
    } else {
      setExtractAudio(false);
    }
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!file) {
      pushToast({ title: 'Select a file first', variant: 'warning' });
      return;
    }

    setUploadPct(0);
    setActiveConvId(null);

    try {
      const job = await createConversion.mutateAsync({
        file,
        targetFormat,
        targetResolution: isAudioTarget ? undefined : targetResolution,
        compress: isAudioTarget ? false : compress,
        extractAudio: isAudioTarget ? true : extractAudio,
        onUploadProgress: (pct) => setUploadPct(pct),
      });
      setActiveConvId(job.id);
      pushToast({ title: 'Conversion queued', description: `Converting ${file.name}`, variant: 'info' });
    } catch (err) {
      const msg = err instanceof APIClientError ? err.message : 'Conversion upload failed';
      pushToast({ title: 'Conversion failed', description: msg, variant: 'error' });
    }
  };

  const handleDownloadOutput = async () => {
    if (!activeConvId) return;
    setDownloading(true);
    try {
      const base = file ? file.name.replace(/\.[^.]+$/, '') : 'converted';
      await conversionService.downloadFile(activeConvId, `${base}.${targetFormat}`);
    } catch (err) {
      const msg = err instanceof APIClientError ? err.message : 'Could not download converted file';
      pushToast({ title: 'Download failed', description: msg, variant: 'error' });
    } finally {
      setDownloading(false);
    }
  };

  return (
    <>
      <PageMeta title="Media Converter" description="Convert authorized video and audio files with FFmpeg." noIndex />

      <div className="mx-auto max-w-container space-y-8 px-4 py-10 sm:px-6">
        <div>
          <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">Media Converter</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Upload your own media file to convert formats, extract audio, resize resolution, or compress video.
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Upload &amp; Conversion Options</CardTitle>
            <CardDescription>
              Original metadata and creator attribution are preserved automatically.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-6">
              <div className="space-y-2">
                <Label htmlFor="conversion-file-input">Local Media File</Label>
                <label
                  htmlFor="conversion-file-input"
                  className="flex cursor-pointer flex-col items-center justify-center rounded-xl border-2 border-dashed border-border bg-background-secondary px-6 py-8 text-center transition-colors hover:border-primary/60"
                >
                  {file ? (
                    <>
                      <FileVideo className="mb-2 h-8 w-8 text-secondary dark:text-cyanHighlight" />
                      <p className="text-sm font-medium text-foreground">{file.name}</p>
                      <p className="mt-1 text-xs text-muted-foreground">{formatBytes(file.size)}</p>
                    </>
                  ) : (
                    <>
                      <UploadCloud className="mb-2 h-8 w-8 text-muted-foreground" />
                      <p className="text-sm font-medium text-foreground">
                        Click to choose an authorized video or audio file
                      </p>
                      <p className="mt-1 text-xs text-muted-foreground">
                        Supports MP4, MOV, MKV, WebM, AVI, MP3, AAC, M4A, WAV, FLAC, OGG
                      </p>
                    </>
                  )}
                  <input
                    id="conversion-file-input"
                    type="file"
                    accept="video/*,audio/*"
                    onChange={handleFileChange}
                    className="sr-only"
                  />
                </label>
              </div>

              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-1.5">
                  <Label htmlFor="target-format">Output Format</Label>
                  <Select
                    id="target-format"
                    value={targetFormat}
                    onChange={(e) => handleFormatChange(e.target.value)}
                  >
                    {CONVERSION_TARGET_FORMATS.map((fmt) => (
                      <option key={fmt.value} value={fmt.value}>
                        {fmt.label}
                      </option>
                    ))}
                  </Select>
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="target-resolution">Resolution Conversion</Label>
                  <Select
                    id="target-resolution"
                    value={targetResolution}
                    disabled={isAudioTarget}
                    onChange={(e) => setTargetResolution(e.target.value)}
                  >
                    {RESOLUTION_PRESETS.map((res) => (
                      <option key={res.value} value={res.value}>
                        {res.label}
                      </option>
                    ))}
                  </Select>
                </div>
              </div>

              <div className="flex flex-wrap gap-6">
                <label className="flex cursor-pointer items-center gap-2 text-sm text-foreground">
                  <input
                    type="checkbox"
                    checked={compress}
                    disabled={isAudioTarget}
                    onChange={(e) => setCompress(e.target.checked)}
                    className="h-4 w-4 rounded border-border accent-primary"
                  />
                  Enable video compression (CRF 28)
                </label>

                <label className="flex cursor-pointer items-center gap-2 text-sm text-foreground">
                  <input
                    type="checkbox"
                    checked={extractAudio}
                    disabled={!isAudioTarget}
                    onChange={(e) => setExtractAudio(e.target.checked)}
                    className="h-4 w-4 rounded border-border accent-primary"
                  />
                  Audio extraction mode
                </label>
              </div>

              {createConversion.isPending && uploadPct > 0 && (
                <div className="space-y-1.5">
                  <div className="flex justify-between text-xs text-muted-foreground">
                    <span>Uploading file...</span>
                    <span>{uploadPct}%</span>
                  </div>
                  <Progress value={uploadPct} label="File upload progress" />
                </div>
              )}

              <div className="flex justify-end">
                <Button type="submit" size="lg" disabled={!file} isLoading={createConversion.isPending}>
                  <Wand2 className="h-4 w-4" aria-hidden="true" />
                  Start Conversion
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>

        {activeConvId && statusQuery.data && (
          <ProcessingStatusCard
            title={file ? `${file.name} → ${targetFormat.toUpperCase()}` : 'Conversion Job'}
            status={statusQuery.data.status}
            progress={statusQuery.data.progress}
            errorMessage={statusQuery.data.error_message}
            onDownloadFile={handleDownloadOutput}
            isDownloadingFile={downloading}
          />
        )}
      </div>
    </>
  );
}

