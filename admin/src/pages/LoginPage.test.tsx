import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import LoginPage from './LoginPage';
import { getAuthConfig } from '../lib/api';

vi.mock('../lib/api', () => ({
  getAuthConfig: vi.fn().mockResolvedValue({ githubOAuthEnabled: true, loginURL: '/auth/github' }),
  saveToken: vi.fn(),
  verifyToken: vi.fn(),
}));

describe('LoginPage', () => {
  it('shows legal links alongside the sign-in statement', async () => {
    vi.mocked(getAuthConfig).mockResolvedValue({ githubOAuthEnabled: true, loginURL: '/auth/github' });

    render(
      <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <LoginPage />
      </MemoryRouter>,
    );

    expect(await screen.findByRole('link', { name: 'Terms' })).toHaveAttribute('href', 'https://semrel.io/legal/terms/');
    expect(document.body).toHaveTextContent(/By signing in, you agree/i);
    expect(screen.getByRole('link', { name: 'Privacy' })).toHaveAttribute('href', 'https://semrel.io/legal/privacy/');
    expect(screen.getByRole('link', { name: 'Imprint' })).toHaveAttribute('href', 'https://semrel.io/legal/imprint/');
  });
});
