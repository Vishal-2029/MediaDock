import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import { HomePage } from '@/pages/Home/HomePage';

describe('HomePage', () => {
  it('renders brand headline, URL input card, and distinguishes available vs planned platforms', () => {
    render(
      <MemoryRouter>
        <HomePage />
      </MemoryRouter>,
    );

    expect(screen.getByText('Your Media. One Simple Destination.')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Paste your media URL here...')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /inspect media/i })).toBeInTheDocument();

    // Available vs Planned badges
    expect(screen.getAllByText('Available').length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText('Planned').length).toBe(2);
  });
});

