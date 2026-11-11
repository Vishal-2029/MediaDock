import type { HTMLAttributes } from 'react';
import { cn } from '@/utils/cn';

export function Skeleton({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('animate-pulse rounded-lg bg-background-secondary', className)} {...props} />;
}

