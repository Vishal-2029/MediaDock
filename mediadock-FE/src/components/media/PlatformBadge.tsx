import { Globe, Instagram, Youtube, Share2 } from 'lucide-react';
import { Badge } from '@/components/common/Badge';
import type { PlatformID } from '@/types/api';

interface PlatformBadgeProps {
  provider: PlatformID | 'unknown';
  label?: string;
}

export function PlatformBadge({ provider, label }: PlatformBadgeProps) {
  switch (provider) {
    case 'instagram':
      return (
        <Badge variant="primary">
          <Instagram className="h-3.5 w-3.5" aria-hidden="true" />
          {label || 'Instagram'}
        </Badge>
      );
    case 'youtube':
      return (
        <Badge variant="warning">
          <Youtube className="h-3.5 w-3.5" aria-hidden="true" />
          {label || 'YouTube (Planned)'}
        </Badge>
      );
    case 'facebook':
      return (
        <Badge variant="warning">
          <Share2 className="h-3.5 w-3.5" aria-hidden="true" />
          {label || 'Facebook (Planned)'}
        </Badge>
      );
    case 'generic':
      return (
        <Badge variant="secondary">
          <Globe className="h-3.5 w-3.5" aria-hidden="true" />
          {label || 'Direct Authorized Media'}
        </Badge>
      );
    case 'external':
      return (
        <Badge variant="primary">
          <Globe className="h-3.5 w-3.5" aria-hidden="true" />
          {label || 'Licensed External Provider'}
        </Badge>
      );
    default:
      return (
        <Badge variant="default">
          <Globe className="h-3.5 w-3.5" aria-hidden="true" />
          {label || 'Unrecognized Source'}
        </Badge>
      );
  }
}

