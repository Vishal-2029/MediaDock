import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { Card, CardContent } from '@/components/common/Card';

interface EmptyStateProps {
  icon: LucideIcon;
  title: string;
  description: string;
  action?: ReactNode;
}

export function EmptyState({ icon: Icon, title, description, action }: EmptyStateProps) {
  return (
    <Card className="border-dashed">
      <CardContent className="flex flex-col items-center justify-center py-14 text-center">
        <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-background-secondary text-secondary dark:text-cyanHighlight">
          <Icon className="h-6 w-6" aria-hidden="true" />
        </div>
        <h3 className="text-base font-semibold text-foreground">{title}</h3>
        <p className="mt-1.5 max-w-md text-sm text-muted-foreground">{description}</p>
        {action && <div className="mt-5">{action}</div>}
      </CardContent>
    </Card>
  );
}

