import { useEffect } from 'react';

interface PageMetaProps {
  title: string;
  description: string;
  noIndex?: boolean;
}

export function PageMeta({ title, description, noIndex = false }: PageMetaProps) {
  useEffect(() => {
    document.title = `${title} | MediaDock`;

    let metaDesc = document.querySelector('meta[name="description"]');
    if (!metaDesc) {
      metaDesc = document.createElement('meta');
      metaDesc.setAttribute('name', 'description');
      document.head.appendChild(metaDesc);
    }
    metaDesc.setAttribute('content', description);

    let robots = document.querySelector('meta[name="robots"]');
    if (noIndex) {
      if (!robots) {
        robots = document.createElement('meta');
        robots.setAttribute('name', 'robots');
        document.head.appendChild(robots);
      }
      robots.setAttribute('content', 'noindex, nofollow');
    } else if (robots) {
      robots.setAttribute('content', 'index, follow');
    }
  }, [title, description, noIndex]);

  return null;
}

