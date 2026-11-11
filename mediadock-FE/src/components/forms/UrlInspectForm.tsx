import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { ClipboardPaste, Search, ShieldAlert, X, KeyRound } from 'lucide-react';
import { Button } from '@/components/common/Button';
import { Input, Label } from '@/components/common/Input';
import { PlatformBadge } from '@/components/media/PlatformBadge';
import { detectPlatformFromUrl } from '@/utils/url';
import { useToastStore } from '@/store/toastStore';

const inspectSchema = z.object({
  url: z
    .string()
    .trim()
    .min(1, 'Please enter a media URL')
    .url('Please enter a valid HTTP or HTTPS URL')
    .refine((val) => val.startsWith('http://') || val.startsWith('https://'), {
      message: 'Only HTTP and HTTPS URLs are supported',
    }),
  accessToken: z.string().optional(),
  ownerAttested: z.boolean().optional(),
});

export type UrlInspectValues = z.infer<typeof inspectSchema>;

interface UrlInspectFormProps {
  initialUrl?: string;
  onSubmit: (values: UrlInspectValues) => void;
  isLoading?: boolean;
  showAuthOptions?: boolean;
}

export function UrlInspectForm({
  initialUrl = '',
  onSubmit,
  isLoading,
  showAuthOptions = true,
}: UrlInspectFormProps) {
  const pushToast = useToastStore((s) => s.push);
  const [showTokenField, setShowTokenField] = useState(false);

  const {
    register,
    handleSubmit,
    watch,
    setValue,
    formState: { errors },
  } = useForm<UrlInspectValues>({
    resolver: zodResolver(inspectSchema),
    defaultValues: {
      url: initialUrl,
      accessToken: '',
      ownerAttested: false,
    },
  });

  const currentUrl = watch('url') || '';
  const detected = detectPlatformFromUrl(currentUrl);

  const handlePaste = async () => {
    try {
      const text = await navigator.clipboard.readText();
      if (text) {
        setValue('url', text.trim(), { shouldValidate: true });
      }
    } catch {
      pushToast({
        title: 'Clipboard unavailable',
        description: 'Please paste the URL manually using Ctrl+V / Cmd+V.',
        variant: 'warning',
      });
    }
  };

  const handleClear = () => {
    setValue('url', '', { shouldValidate: false });
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
      <div className="space-y-2">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <Label htmlFor="media-url-input" className="text-sm font-medium">
            Authorized Media URL
          </Label>
          {currentUrl.trim() !== '' && <PlatformBadge provider={detected.id} label={detected.label} />}
        </div>

        <div className="flex flex-col gap-2 sm:flex-row">
          <div className="relative flex-1">
            <Input
              id="media-url-input"
              type="url"
              placeholder="Paste your media URL here..."
              aria-invalid={Boolean(errors.url)}
              className="h-12 pr-20 text-base"
              {...register('url')}
            />
            <div className="absolute right-2 top-1/2 flex -translate-y-1/2 items-center gap-1">
              {currentUrl ? (
                <button
                  type="button"
                  onClick={handleClear}
                  className="rounded-md p-1.5 text-muted-foreground hover:bg-card hover:text-foreground"
                  aria-label="Clear URL"
                  title="Clear"
                >
                  <X className="h-4 w-4" />
                </button>
              ) : (
                <button
                  type="button"
                  onClick={handlePaste}
                  className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-secondary hover:bg-card dark:text-cyanHighlight"
                  aria-label="Paste URL from clipboard"
                >
                  <ClipboardPaste className="h-3.5 w-3.5" />
                  Paste
                </button>
              )}
            </div>
          </div>

          <Button type="submit" size="lg" isLoading={isLoading} className="h-12 shrink-0 px-6">
            <Search className="h-4 w-4" aria-hidden="true" />
            Inspect Media
          </Button>
        </div>

        {errors.url && (
          <p role="alert" className="text-xs font-medium text-destructive">
            {errors.url.message}
          </p>
        )}
      </div>

      {showAuthOptions && (detected.id === 'instagram' || detected.id === 'generic' || showTokenField) && (
        <div className="space-y-3 rounded-lg border border-border bg-background-secondary p-4">
          {detected.id === 'instagram' && !showTokenField && (
            <p className="text-xs leading-relaxed text-muted-foreground">
              Instagram blocks downloading Reels from other accounts without creator Graph API permission. If this is
              your own reel and you have a Meta Graph API token, click{' '}
              <span className="font-medium text-foreground">Provide creator OAuth token</span> below — or use{' '}
              <span className="font-medium text-foreground">Converters</span> to convert saved video/audio files without
              login.
            </p>
          )}

          {showTokenField && (
            <div className="space-y-1.5">
              <Label htmlFor="platform-oauth-token" className="flex items-center gap-1.5 text-xs">
                <KeyRound className="h-3.5 w-3.5 text-secondary dark:text-cyanHighlight" />
                Official Instagram Graph API Token (optional, for your own account)
              </Label>
              <Input
                id="platform-oauth-token"
                type="password"
                autoComplete="off"
                placeholder="Paste per-request Meta OAuth token (optional)"
                {...register('accessToken')}
              />
            </div>
          )}

          {detected.id === 'generic' && (
            <label className="flex cursor-pointer items-start gap-2.5 text-xs text-muted-foreground">
              <input
                type="checkbox"
                className="mt-0.5 h-4 w-4 rounded border-border accent-primary"
                {...register('ownerAttested')}
              />
              <span>
                I confirm I own or hold explicit permission to retrieve and process this direct media URL.
              </span>
            </label>
          )}
        </div>
      )}

      <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
        <p className="flex items-center gap-1.5">
          <ShieldAlert className="h-3.5 w-3.5 shrink-0 text-secondary dark:text-cyanHighlight" aria-hidden="true" />
          No account login required — only retrieve or convert media you own or have permission to access.
        </p>
        {showAuthOptions && (
          <button
            type="button"
            onClick={() => setShowTokenField((v) => !v)}
            className="font-medium text-secondary hover:underline dark:text-cyanHighlight"
          >
            {showTokenField ? 'Hide creator OAuth token' : 'Provide creator OAuth token'}
          </button>
        )}
      </div>
    </form>
  );
}

