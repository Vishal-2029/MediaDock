import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ProcessingStatusCard } from '@/components/media/ProcessingStatusCard';

describe('ProcessingStatusCard', () => {
  it('displays Waiting state for pending jobs', () => {
    render(<ProcessingStatusCard title="My Clip" status="pending" progress={0} />);
    expect(screen.getByText('Waiting')).toBeInTheDocument();
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0');
  });

  it('displays Processing stage and real progress percentage', () => {
    render(<ProcessingStatusCard title="My Clip" status="processing" progress={65} />);
    expect(screen.getByText('Processing')).toBeInTheDocument();
    expect(screen.getByText('65%')).toBeInTheDocument();
  });

  it('renders Download File action when completed', () => {
    const onDownload = vi.fn();
    render(
      <ProcessingStatusCard
        title="My Clip"
        status="completed"
        progress={100}
        onDownloadFile={onDownload}
      />,
    );
    const btn = screen.getByRole('button', { name: /download file/i });
    fireEvent.click(btn);
    expect(onDownload).toHaveBeenCalledTimes(1);
  });
});

