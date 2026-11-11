import { useNavigate } from 'react-router-dom';
import { motion } from 'framer-motion';
import {
  CheckCircle2,
  Clock,
  Cpu,
  FileAudio,
  History,
  Layers,
  Lock,
  PlaySquare,
  SearchCheck,
  ShieldCheck,
  Sparkles,
} from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/common/Card';
import { Badge } from '@/components/common/Badge';
import { PageMeta } from '@/components/common/PageMeta';
import { UrlInspectForm, type UrlInspectValues } from '@/components/forms/UrlInspectForm';
import { PLATFORM_CATALOG } from '@/constants/platforms';

const HOW_IT_WORKS = [
  {
    step: '01',
    icon: SearchCheck,
    title: 'Paste your authorized media URL',
    description:
      'Enter a supported link or upload your own media file. MediaDock identifies the source platform and validates URL safety.',
  },
  {
    step: '02',
    icon: Layers,
    title: 'Inspect available media and formats',
    description:
      'Verify ownership or official OAuth authorization and inspect metadata, duration, thumbnail, and permitted formats.',
  },
  {
    step: '03',
    icon: PlaySquare,
    title: 'Preview and download your permitted media',
    description:
      'Queue conversion or retrieval jobs, track live progress, preview output, and download securely.',
  },
];

const FEATURES = [
  {
    icon: Cpu,
    title: 'Fast media processing',
    description: 'Redis-backed asynchronous worker queues handle retrieval and FFmpeg jobs without blocking requests.',
  },
  {
    icon: Layers,
    title: 'Multiple media formats',
    description: 'Convert authorized video and audio across MP4, MOV, MKV, WebM, MP3, AAC, M4A, WAV, FLAC, and OGG.',
  },
  {
    icon: PlaySquare,
    title: 'Video preview',
    description: 'Inspect thumbnails, duration, file size, and preview saved media items directly in your browser.',
  },
  {
    icon: FileAudio,
    title: 'Audio conversion',
    description: 'Extract authorized audio tracks or transcode recordings into lossless and compressed formats.',
  },
  {
    icon: History,
    title: 'Download history',
    description: 'Searchable, paginated audit trail of your completed downloads with configurable retention cleanup.',
  },
  {
    icon: Lock,
    title: 'Privacy-focused processing',
    description: 'SSRF-protected retrieval, automatic temporary file expiration, and zero password or cookie harvesting.',
  },
];

export function HomePage() {
  const navigate = useNavigate();

  const handleInspectSubmit = (values: UrlInspectValues) => {
    const params = new URLSearchParams({ url: values.url });
    if (values.ownerAttested) {
      params.set('owner_attested', 'true');
    }
    navigate(`/downloader?${params.toString()}`, {
      state: values.accessToken ? { accessToken: values.accessToken } : undefined,
    });
  };

  return (
    <>
      <PageMeta
        title="Your Media. One Simple Destination"
        description="Retrieve, preview, convert, and manage your authorized media from one simple platform."
      />

      {/* Section B: Hero */}
      <section className="relative overflow-hidden border-b border-border bg-gradient-to-b from-background-secondary/70 via-background to-background py-16 sm:py-24">
        <div className="mx-auto max-w-container px-4 sm:px-6">
          <motion.div
            initial={{ opacity: 0, y: 12 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.35 }}
            className="mx-auto max-w-3xl text-center"
          >
            <div className="mb-5 inline-flex items-center gap-2 rounded-full border border-border bg-card px-3.5 py-1 text-xs font-medium text-secondary dark:text-cyanHighlight">
              <Sparkles className="h-3.5 w-3.5" aria-hidden="true" />
              Multi-Platform Authorized Media Management
            </div>

            <h1 className="text-3xl font-bold tracking-tight text-foreground sm:text-5xl">
              Your Media. One Simple Destination.
            </h1>

            <p className="mx-auto mt-4 max-w-2xl text-base text-muted-foreground sm:text-lg">
              Retrieve, preview, convert, and manage your authorized media from one simple platform.
            </p>
          </motion.div>

          <motion.div
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.4, delay: 0.08 }}
            className="mx-auto mt-10 max-w-3xl"
          >
            <Card className="border-border/80 shadow-xl">
              <CardContent className="p-5 sm:p-7">
                <UrlInspectForm onSubmit={handleInspectSubmit} showAuthOptions={false} />
              </CardContent>
            </Card>
          </motion.div>
        </div>
      </section>

      {/* Section C: Supported Platforms */}
      <section id="platforms" className="mx-auto max-w-container px-4 py-16 sm:px-6">
        <div className="mb-10 max-w-2xl">
          <h2 className="text-2xl font-bold tracking-tight sm:text-3xl">Supported Platforms</h2>
          <p className="mt-2 text-sm text-muted-foreground sm:text-base">
            MediaDock strictly uses official APIs and authorized retrieval workflows. Planned integrations are
            clearly labeled and remain disabled until compliant access is enabled.
          </p>
        </div>

        <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
          {PLATFORM_CATALOG.map((platform) => {
            const isAvailable = platform.status === 'available';
            return (
              <Card key={platform.id} className="flex flex-col justify-between">
                <CardHeader>
                  <div className="flex items-center justify-between gap-2">
                    <CardTitle className="text-base">{platform.name}</CardTitle>
                    {isAvailable ? (
                      <Badge variant="success">
                        <CheckCircle2 className="h-3 w-3" /> Available
                      </Badge>
                    ) : (
                      <Badge variant="warning">
                        <Clock className="h-3 w-3" /> Planned
                      </Badge>
                    )}
                  </div>
                  <CardDescription className="text-xs font-medium text-secondary dark:text-cyanHighlight">
                    {platform.authMethod}
                  </CardDescription>
                </CardHeader>
                <CardContent className="pt-0">
                  <p className="text-sm text-muted-foreground">{platform.description}</p>
                  <div className="mt-4 flex flex-wrap gap-1.5">
                    {platform.domains.map((domain) => (
                      <span
                        key={domain}
                        className="rounded border border-border bg-background-secondary px-2 py-0.5 text-[11px] text-muted-foreground"
                      >
                        {domain}
                      </span>
                    ))}
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      </section>

      {/* Section D: How It Works */}
      <section className="border-y border-border bg-background-secondary/50 py-16">
        <div className="mx-auto max-w-container px-4 sm:px-6">
          <div className="mb-10 max-w-2xl">
            <h2 className="text-2xl font-bold tracking-tight sm:text-3xl">How It Works</h2>
            <p className="mt-2 text-sm text-muted-foreground sm:text-base">
              Three transparent steps from URL inspection to verified download.
            </p>
          </div>

          <div className="grid gap-6 md:grid-cols-3">
            {HOW_IT_WORKS.map((item) => {
              const Icon = item.icon;
              return (
                <Card key={item.step}>
                  <CardHeader>
                    <div className="mb-2 flex items-center justify-between">
                      <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/15 text-primary">
                        <Icon className="h-5 w-5" aria-hidden="true" />
                      </div>
                      <span className="text-xs font-bold tracking-widest text-muted-foreground">
                        STEP {item.step}
                      </span>
                    </div>
                    <CardTitle className="text-base">{item.title}</CardTitle>
                  </CardHeader>
                  <CardContent className="pt-0">
                    <p className="text-sm text-muted-foreground">{item.description}</p>
                  </CardContent>
                </Card>
              );
            })}
          </div>
        </div>
      </section>

      {/* Section E: Features */}
      <section id="features" className="mx-auto max-w-container px-4 py-16 sm:px-6">
        <div className="mb-10 flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
          <div className="max-w-2xl">
            <h2 className="text-2xl font-bold tracking-tight sm:text-3xl">Built for Authorized Workflows</h2>
            <p className="mt-2 text-sm text-muted-foreground sm:text-base">
              Every capability is backed by our Go Fiber REST API, PostgreSQL, Redis worker queues, and FFmpeg.
            </p>
          </div>
          <div className="inline-flex items-center gap-2 text-xs text-muted-foreground">
            <ShieldCheck className="h-4 w-4 text-success" />
            Original creator attribution preserved
          </div>
        </div>

        <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {FEATURES.map((feat) => {
            const Icon = feat.icon;
            return (
              <Card key={feat.title}>
                <CardHeader>
                  <div className="mb-2 flex h-10 w-10 items-center justify-center rounded-lg bg-secondary/15 text-secondary dark:text-cyanHighlight">
                    <Icon className="h-5 w-5" aria-hidden="true" />
                  </div>
                  <CardTitle className="text-base">{feat.title}</CardTitle>
                </CardHeader>
                <CardContent className="pt-0">
                  <p className="text-sm text-muted-foreground">{feat.description}</p>
                </CardContent>
              </Card>
            );
          })}
        </div>
      </section>
    </>
  );
}

