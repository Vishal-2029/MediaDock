import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { MediaPreviewCard } from '@/components/media/MediaPreviewCard';
import type { MediaInspectResponse } from '@/types/api';

const sampleMedia: MediaInspectResponse = {
  media_id: 'media-123',
  source_id: 'source-123',
  provider: 'instagram',
  source_url: 'https://www.instagram.com/p/C9Demo/',
  title: 'Creator Owned Reel',
  thumbnail_url: 'https://cdn.example.com/thumb.jpg',
  duration: 42,
  original_format: 'mp4',
  file_size: 4_194_304,
  available_formats: [
    { format: 'mp4', quality: '1080p', container: 'mp4', description: 'Original HD MP4' },
    { format: 'mp3', quality: 'audio', container: 'mp3', description: 'Audio Track MP3', audio_only: true },
  ],
  authorization: {
    authorized: true,
    authorization_method: 'oauth',
    reason: 'Verified owner via official Instagram Graph API',
  },
  created_at: '2026-11-11T10:00:00Z',
};

describe('MediaPreviewCard', () => {
  it('displays metadata, switches formats, and invokes onStartDownload explicitly on click', () => {
    const onStartDownload = vi.fn();

    render(<MediaPreviewCard media={sampleMedia} onStartDownload={onStartDownload} />);

    expect(screen.getByText('Creator Owned Reel')).toBeInTheDocument();
    expect(screen.getByText(/Authorized \(oauth\)/i)).toBeInTheDocument();

    // Does not auto-trigger download on mount
    expect(onStartDownload).not.toHaveBeenCalled();

    const formatSelect = screen.getByLabelText('Available Format');
    fireEvent.change(formatSelect, { target: { value: 'mp3' } });

    fireEvent.click(screen.getByRole('button', { name: /download media/i }));
    expect(onStartDownload).toHaveBeenCalledTimes(1);
    expect(onStartDownload).toHaveBeenCalledWith('mp3', 'audio');
  });

  it('disables download and shows compliance guidance for preview-only media', () => {
    const onStartDownload = vi.fn();
    const previewOnlyMedia: MediaInspectResponse = {
      ...sampleMedia,
      downloadable: false,
      download_blocked_reason: 'Public oEmbed preview only; licensed download requires creator OAuth.',
      available_formats: [],
      authorization: {
        authorized: false,
        preview_only: true,
        authorization_method: 'meta_oembed',
        reason: 'Public oEmbed preview only; licensed download requires creator OAuth.',
      },
    };

    render(<MediaPreviewCard media={previewOnlyMedia} onStartDownload={onStartDownload} />);

    expect(screen.getByText('Preview Only')).toBeInTheDocument();
    expect(screen.getByText(/Public oEmbed preview only/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /download media/i })).toBeDisabled();
  });
});
