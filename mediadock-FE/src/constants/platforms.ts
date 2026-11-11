import type { PlatformID } from '@/types/api';

export interface PlatformCatalogItem {
  id: PlatformID;
  name: string;
  status: 'available' | 'planned';
  authMethod: string;
  description: string;
  domains: string[];
}

export const PLATFORM_CATALOG: PlatformCatalogItem[] = [
  {
    id: 'instagram',
    name: 'Instagram',
    status: 'available',
    authMethod: 'Official Meta Graph API (OAuth)',
    description:
      'Retrieve and process your own authorized Instagram posts and reels using an official Meta OAuth access token.',
    domains: ['instagram.com', 'www.instagram.com', 'm.instagram.com'],
  },
  {
    id: 'generic',
    name: 'Direct Authorized Sources',
    status: 'available',
    authMethod: 'Owner Attestation + SSRF Verification',
    description:
      'Inspect and process direct user-owned media URLs (MP4, MOV, WebM, MP3, WAV, PNG, JPEG) with explicit permission confirmation.',
    domains: ['Direct Media URLs'],
  },
  {
    id: 'youtube',
    name: 'YouTube',
    status: 'planned',
    authMethod: 'Future Authorized Creator Integration',
    description:
      'Reserved for future authorized creator workflows. General-purpose downloading is not enabled; use YouTube Studio or upload originals directly.',
    domains: ['youtube.com', 'www.youtube.com', 'youtu.be'],
  },
  {
    id: 'facebook',
    name: 'Facebook',
    status: 'planned',
    authMethod: 'Future Meta Authorized Integration',
    description:
      'Reserved for future authorized Meta integrations. Export owned media via Account Center or upload directly to the MediaDock Converter.',
    domains: ['facebook.com', 'www.facebook.com', 'fb.watch'],
  },
];

export const CONVERSION_TARGET_FORMATS = [
  { value: 'mp4', label: 'MP4 (H.264 Video)', kind: 'video' },
  { value: 'mov', label: 'MOV (QuickTime)', kind: 'video' },
  { value: 'mkv', label: 'MKV (Matroska)', kind: 'video' },
  { value: 'webm', label: 'WebM (VP9)', kind: 'video' },
  { value: 'mp3', label: 'MP3 (Audio)', kind: 'audio' },
  { value: 'aac', label: 'AAC (Audio)', kind: 'audio' },
  { value: 'm4a', label: 'M4A (Audio)', kind: 'audio' },
  { value: 'wav', label: 'WAV (Lossless Audio)', kind: 'audio' },
  { value: 'flac', label: 'FLAC (Lossless Audio)', kind: 'audio' },
  { value: 'ogg', label: 'OGG (Vorbis)', kind: 'audio' },
] as const;

export const RESOLUTION_PRESETS = [
  { value: 'original', label: 'Original Resolution' },
  { value: '1080p', label: '1080p (Full HD)' },
  { value: '720p', label: '720p (HD)' },
  { value: '480p', label: '480p (SD)' },
  { value: '360p', label: '360p (Compact)' },
] as const;

