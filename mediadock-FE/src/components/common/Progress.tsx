import { cn } from '@/utils/cn';

interface ProgressProps {
  value?: number | null;
  className?: string;
  label?: string;
}

export function Progress({ value, className, label }: ProgressProps) {
  const hasNumeric = typeof value === 'number' && !Number.isNaN(value);
  const clamped = hasNumeric ? Math.min(100, Math.max(0, value)) : 0;

  return (
    <div
      role="progressbar"
      aria-label={label || 'Operation progress'}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={hasNumeric ? clamped : undefined}
      className={cn('h-2.5 w-full overflow-hidden rounded-full bg-background-secondary', className)}
    >
      <div
        className="h-full rounded-full bg-primary transition-all duration-300"
        style={{ width: `${clamped}%` }}
      />
    </div>
  );
}

