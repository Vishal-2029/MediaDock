import { useState } from 'react';
import { Dialog } from '@/components/common/Dialog';
import { Button } from '@/components/common/Button';

type PolicyTopic = 'about' | 'privacy' | 'terms' | 'copyright' | 'contact' | null;

const POLICY_COPY: Record<Exclude<PolicyTopic, null>, { title: string; body: string }> = {
  about: {
    title: 'About MediaDock',
    body: 'MediaDock is a unified platform for retrieving, previewing, converting, and managing media you own or have explicit permission to process via authorized APIs.',
  },
  privacy: {
    title: 'Privacy Policy',
    body: 'MediaDock never harvests platform passwords or session cookies. Temporary processed artifacts expire automatically based on retention policies, and user sessions use secure HTTP-only cookies.',
  },
  terms: {
    title: 'Terms of Service',
    body: 'By using MediaDock, you confirm that you own the submitted media or hold explicit permission from the copyright holder. Automated scraping or circumvention of access controls is strictly prohibited.',
  },
  copyright: {
    title: 'Copyright Policy',
    body: 'MediaDock preserves original creator attribution and metadata and never adds MediaDock watermarks or removes third-party copyright notices.',
  },
  contact: {
    title: 'Contact MediaDock',
    body: 'For API integration questions, compliance inquiries, or support, reach the MediaDock engineering team at support@mediadock.app.',
  },
};

export function Footer() {
  const [topic, setTopic] = useState<PolicyTopic>(null);

  return (
    <footer className="border-t border-border bg-background-secondary">
      <div className="mx-auto flex max-w-container flex-col gap-6 px-4 py-10 sm:px-6 md:flex-row md:items-center md:justify-between">
        <div className="flex items-center gap-3">
          <img src="/logo.svg" alt="MediaDock logo" className="h-8 w-8" />
          <div>
            <p className="text-sm font-semibold text-foreground">MediaDock</p>
            <p className="text-xs text-muted-foreground">Your Media. One Simple Destination.</p>
          </div>
        </div>

        <nav aria-label="Footer links" className="flex flex-wrap gap-x-6 gap-y-2 text-sm text-muted-foreground">
          <button type="button" onClick={() => setTopic('about')} className="hover:text-foreground">
            About MediaDock
          </button>
          <button type="button" onClick={() => setTopic('privacy')} className="hover:text-foreground">
            Privacy Policy
          </button>
          <button type="button" onClick={() => setTopic('terms')} className="hover:text-foreground">
            Terms of Service
          </button>
          <button type="button" onClick={() => setTopic('copyright')} className="hover:text-foreground">
            Copyright Policy
          </button>
          <button type="button" onClick={() => setTopic('contact')} className="hover:text-foreground">
            Contact
          </button>
          <a href="/#platforms" className="hover:text-foreground">
            Supported Platforms
          </a>
        </nav>
      </div>

      <Dialog
        open={topic !== null}
        onClose={() => setTopic(null)}
        title={topic ? POLICY_COPY[topic].title : ''}
        footer={
          <Button variant="primary" onClick={() => setTopic(null)}>
            Close
          </Button>
        }
      >
        <p className="text-sm leading-relaxed text-muted-foreground">
          {topic ? POLICY_COPY[topic].body : ''}
        </p>
      </Dialog>
    </footer>
  );
}

