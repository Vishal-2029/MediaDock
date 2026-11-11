import { Component, type ErrorInfo, type ReactNode } from 'react';
import { AlertTriangle } from 'lucide-react';
import { Button } from '@/components/common/Button';

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  message: string;
}

export class ErrorBoundary extends Component<Props, State> {
  public state: State = { hasError: false, message: '' };

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, message: error.message || 'An unexpected rendering error occurred.' };
  }

  public componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('MediaDock ErrorBoundary caught:', error, info);
  }

  public render() {
    if (this.state.hasError) {
      return (
        <div className="flex min-h-screen flex-col items-center justify-center bg-background px-4 text-center">
          <div className="max-w-md rounded-xl border border-border bg-card p-8">
            <AlertTriangle className="mx-auto mb-4 h-10 w-10 text-warning" />
            <h1 className="text-xl font-semibold text-foreground">Something went wrong</h1>
            <p className="mt-2 text-sm text-muted-foreground">{this.state.message}</p>
            <Button
              className="mt-6"
              onClick={() => {
                this.setState({ hasError: false, message: '' });
                window.location.assign('/');
              }}
            >
              Return Home
            </Button>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}

