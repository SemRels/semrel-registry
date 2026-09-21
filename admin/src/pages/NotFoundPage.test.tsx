import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import NotFoundPage from './NotFoundPage';

// A mistyped plugin link used to redirect silently to the home page, which
// reads as "this plugin was deleted".
describe('NotFoundPage', () => {
  it('names the missing path and offers a way onward', () => {
    render(
      <MemoryRouter initialEntries={['/plugins/typo-here']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <NotFoundPage />
      </MemoryRouter>,
    );

    expect(screen.getByRole('heading', { name: /page not found/i })).toBeInTheDocument();
    expect(document.body).toHaveTextContent('/plugins/typo-here');
    expect(screen.getByRole('link', { name: /browse the registry/i })).toHaveAttribute('href', '/');
  });
});
