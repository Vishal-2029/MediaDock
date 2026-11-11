import type { ReactNode } from 'react';
import { Navbar } from '@/components/navigation/Navbar';
import { Footer } from '@/components/layout/Footer';

interface AppLayoutProps {
  children: ReactNode;
}

export function AppLayout({ children }: AppLayoutProps) {
  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <Navbar />
      <main className="flex-1">{children}</main>
      <Footer />
    </div>
  );
}

