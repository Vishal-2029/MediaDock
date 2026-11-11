import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { UrlInspectForm } from '@/components/forms/UrlInspectForm';

describe('UrlInspectForm', () => {
  it('shows validation error for invalid URL and submits valid URL', async () => {
    const onSubmit = vi.fn();

    render(<UrlInspectForm onSubmit={onSubmit} />);

    const input = screen.getByPlaceholderText('Paste your media URL here...');
    fireEvent.change(input, { target: { value: 'not-a-valid-url' } });
    fireEvent.click(screen.getByRole('button', { name: /inspect media/i }));

    expect(await screen.findByRole('alert')).toHaveTextContent(/valid http or https url/i);
    expect(onSubmit).not.toHaveBeenCalled();

    // Clear button clears the input
    fireEvent.click(screen.getByRole('button', { name: /clear url/i }));
    expect(input).toHaveValue('');

    // Valid Instagram URL triggers submit
    fireEvent.change(input, { target: { value: 'https://www.instagram.com/p/C9DemoPost123/' } });
    fireEvent.click(screen.getByRole('button', { name: /inspect media/i }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledTimes(1);
    });
    expect(onSubmit.mock.calls[0][0].url).toBe('https://www.instagram.com/p/C9DemoPost123/');
  });
});
