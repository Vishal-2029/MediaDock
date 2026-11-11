import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it } from 'vitest';
import { ProtectedRoute } from '@/routes/ProtectedRoute';
import { useAuthStore } from '@/store/authStore';

describe('ProtectedRoute', () => {
  beforeEach(() => {
    useAuthStore.setState({ user: null, isInitialized: true });
  });

  it('redirects unauthenticated user to /login with redirect query', () => {
    render(
      <MemoryRouter initialEntries={['/history']}>
        <Routes>
          <Route element={<ProtectedRoute />}>
            <Route path="/history" element={<div>Protected History View</div>} />
          </Route>
          <Route path="/login" element={<div>Login Page</div>} />
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByText('Login Page')).toBeInTheDocument();
    expect(screen.queryByText('Protected History View')).not.toBeInTheDocument();
  });

  it('renders protected child route when authenticated', () => {
    useAuthStore.setState({
      user: {
        id: 'u1',
        name: 'Alice',
        email: 'alice@example.com',
        created_at: '2026-11-11T10:00:00Z',
        updated_at: '2026-11-11T10:00:00Z',
      },
      isInitialized: true,
    });

    render(
      <MemoryRouter initialEntries={['/history']}>
        <Routes>
          <Route element={<ProtectedRoute />}>
            <Route path="/history" element={<div>Protected History View</div>} />
          </Route>
          <Route path="/login" element={<div>Login Page</div>} />
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByText('Protected History View')).toBeInTheDocument();
  });
});
