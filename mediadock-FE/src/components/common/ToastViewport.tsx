import { AnimatePresence, motion } from 'framer-motion';
import { AlertCircle, CheckCircle2, Info, AlertTriangle, X } from 'lucide-react';
import { useToastStore, type ToastVariant } from '@/store/toastStore';

const ICON_MAP: Record<ToastVariant, typeof Info> = {
  info: Info,
  success: CheckCircle2,
  warning: AlertTriangle,
  error: AlertCircle,
};

const BORDER_MAP: Record<ToastVariant, string> = {
  info: 'border-secondary/50 text-secondary dark:text-cyanHighlight',
  success: 'border-success/50 text-success',
  warning: 'border-warning/50 text-warning',
  error: 'border-destructive/50 text-destructive',
};

export function ToastViewport() {
  const toasts = useToastStore((s) => s.toasts);
  const dismiss = useToastStore((s) => s.dismiss);

  return (
    <div
      aria-live="polite"
      className="pointer-events-none fixed bottom-4 right-4 z-50 flex w-full max-w-sm flex-col gap-2 px-4 sm:px-0"
    >
      <AnimatePresence>
        {toasts.map((toast) => {
          const Icon = ICON_MAP[toast.variant];
          return (
            <motion.div
              key={toast.id}
              initial={{ opacity: 0, y: 12 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: 8 }}
              transition={{ duration: 0.18 }}
              className={`pointer-events-auto flex items-start gap-3 rounded-xl border bg-card p-4 shadow-lg ${BORDER_MAP[toast.variant]}`}
            >
              <Icon className="mt-0.5 h-5 w-5 shrink-0" aria-hidden="true" />
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium text-foreground">{toast.title}</p>
                {toast.description && (
                  <p className="mt-0.5 text-xs text-muted-foreground">{toast.description}</p>
                )}
              </div>
              <button
                type="button"
                onClick={() => dismiss(toast.id)}
                className="rounded p-1 text-muted-foreground hover:text-foreground"
                aria-label="Dismiss notification"
              >
                <X className="h-4 w-4" />
              </button>
            </motion.div>
          );
        })}
      </AnimatePresence>
    </div>
  );
}

