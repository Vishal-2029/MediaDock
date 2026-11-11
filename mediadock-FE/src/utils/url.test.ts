import { describe, expect, it } from 'vitest';
import { detectPlatformFromUrl } from '@/utils/url';
import { formatBytes, formatDuration } from '@/utils/format';

describe('detectPlatformFromUrl', () => {
  it('identifies supported Instagram URLs', () => {
    const res = detectPlatformFromUrl('https://www.instagram.com/reel/Cxyz123/');
    expect(res.id).toBe('instagram');
    expect(res.supportedNow).toBe(true);
    expect(res.isValidUrl).toBe(true);
  });

  it('marks YouTube and Facebook as planned rather than operational', () => {
    const yt = detectPlatformFromUrl('https://www.youtube.com/watch?v=dQw4w9WgXcQ');
    expect(yt.id).toBe('youtube');
    expect(yt.supportedNow).toBe(false);

    const fb = detectPlatformFromUrl('https://www.facebook.com/watch/?v=123');
    expect(fb.id).toBe('facebook');
    expect(fb.supportedNow).toBe(false);
  });

  it('identifies direct media URLs', () => {
    const direct = detectPlatformFromUrl('https://cdn.example.com/clip.mp4');
    expect(direct.id).toBe('generic');
    expect(direct.supportedNow).toBe(true);
  });

  it('rejects non-HTTP protocols', () => {
    const invalid = detectPlatformFromUrl('javascript:alert(1)');
    expect(invalid.isValidUrl).toBe(false);
  });
});

describe('format helpers', () => {
  it('formats byte sizes and durations cleanly', () => {
    expect(formatBytes(0)).toBe('—');
    expect(formatBytes(2048)).toBe('2.0 KB');
    expect(formatDuration(125)).toBe('2:05');
    expect(formatDuration(3661)).toBe('1:01:01');
  });
});

