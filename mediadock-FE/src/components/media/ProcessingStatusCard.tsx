import { CheckCircle2, Clock, Download, Loader2, XCircle, AlertTriangle } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/common/Card';
import { Badge } from '@/components/common/Badge';
import { Button } from '@/components/common/Button';
import { Progress } from '@/components/common/Progress';
import type { JobStatus } from '@/types/api';

interface ProcessingStatusCardProps {
  title: string;
  status: JobStatus;
  progress: number | null | undefined;
  errorMessage?: string;
  onDownloadFile?: () => void;
  isDownloadingFile?: boolean;
}

function resolveStageLabel(status: JobStatus, progress?: number | null): string {
  switch (status) {
    case 'pending':
      return 'Waiting';
    case 'processing':
      if (typeof progress === 'number') {
        if (progress < 25) return 'Validating';
        if (progress < 60) return 'Retrieving';
      }
      return 'Processing';
    case 'completed':
      return 'Completed';
    case 'failed':
      return 'Failed';
    case 'expired':
      return 'Expired';
  }
}

export function ProcessingStatusCard({
  title,
  status,
  progress,
  errorMessage,
  onDownloadFile,
  isDownloadingFile,
}: ProcessingStatusCardProps) {
  const stageLabel = resolveStageLabel(status, progress);
  const hasProgress = typeof progress === 'number' && !Number.isNaN(progress);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between gap-4 pb-3">
        <div className="min-w-0">
          <CardTitle className="truncate text-base">{title}</CardTitle>
        </div>
        {status === 'completed' && (
          <Badge variant="success">
            <CheckCircle2 className="h-3.5 w-3.5" /> {stageLabel}
          </Badge>
        )}
        {(status === 'pending' || status === 'processing') && (
          <Badge variant="secondary">
            {status === 'pending' ? (
              <Clock className="h-3.5 w-3.5" />
            ) : (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            )}
            {stageLabel}
          </Badge>
        )}
        {status === 'failed' && (
          <Badge variant="error">
            <XCircle className="h-3.5 w-3.5" /> {stageLabel}
          </Badge>
        )}
        {status === 'expired' && (
          <Badge variant="warning">
            <AlertTriangle className="h-3.5 w-3.5" /> {stageLabel}
          </Badge>
        )}
      </CardHeader>

      <CardContent className="space-y-4">
        <div className="space-y-2">
          <div className="flex items-center justify-between text-xs text-muted-foreground">
            <span>Stage: {stageLabel}</span>
            {hasProgress && <span>{progress}%</span>}
          </div>
          <Progress value={hasProgress ? progress : 0} label={`${title} ${stageLabel}`} />
        </div>

        {errorMessage && (
          <p className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {errorMessage}
          </p>
        )}

        {status === 'completed' && onDownloadFile && (
          <div className="flex justify-end">
            <Button variant="primary" onClick={onDownloadFile} isLoading={isDownloadingFile}>
              <Download className="h-4 w-4" aria-hidden="true" />
              Download File
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

