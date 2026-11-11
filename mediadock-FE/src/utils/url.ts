import type { PlatformID } from '@/types/api';

export interface DetectedPlatform {
  id: PlatformID | 'unknown';
  label: string;
  supportedNow: boolean;
  isValidUrl: boolean;
}

const DIRECT_MEDIA_EXT = /\.(mp4|mov|mkv|webm|avi|mp3|aac|m4a|wav|flac|ogg|jpg|jpeg|png|webp)$/i;

export function detectPlatformFromUrl(rawUrl: string): DetectedPlatform {
  const trimmed = rawUrl.trim();
  if (!trimmed) {
    return { id: 'unknown', label: 'Awaiting URL', supportedNow: false, isValidUrl: false };
  }

  let parsed: URL;
  try {
    parsed = new URL(trimmed);
  } catch {
    return { id: 'unknown', label: 'Invalid URL', supportedNow: false, isValidUrl: false };
  }

  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    return { id: 'unknown', label: 'Unsupported Protocol', supportedNow: false, isValidUrl: false };
  }

  const host = parsed.hostname.toLowerCase();

  if (host === 'instagram.com' || host === 'www.instagram.com' || host === 'm.instagram.com') {
    return { id: 'instagram', label: 'Instagram', supportedNow: true, isValidUrl: true };
  }
  if (host === 'youtube.com' || host === 'www.youtube.com' || host === 'm.youtube.com' || host === 'youtu.be') {
    return { id: 'youtube', label: 'YouTube (Planned)', supportedNow: false, isValidUrl: true };
  }
  if (host === 'facebook.com' || host === 'www.facebook.com' || host === 'm.facebook.com' || host === 'fb.watch') {
    return { id: 'facebook', label: 'Facebook (Planned)', supportedNow: false, isValidUrl: true };
  }
  if (DIRECT_MEDIA_EXT.test(parsed.pathname)) {
    return { id: 'generic', label: 'Direct Media URL', supportedNow: true, isValidUrl: true };
  }

  return { id: 'unknown', label: 'Unrecognized Source', supportedNow: false, isValidUrl: true };
}

