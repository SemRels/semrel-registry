import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import CopyButton from './CopyButton';

/** navigator.clipboard is a getter-only property in jsdom. */
function stubClipboard(writeText: () => Promise<void>) {
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText: vi.fn(writeText) },
    configurable: true,
  });
  return navigator.clipboard.writeText as ReturnType<typeof vi.fn>;
}

describe('CopyButton', () => {
  it('names what it copies, rather than just "Copy"', () => {
    stubClipboard(() => Promise.resolve());
    render(<CopyButton text="semrel plugin install @semrel/provider-github" label="Copy install command" />);

    expect(screen.getByRole('button', {
      name: 'Copy install command: semrel plugin install @semrel/provider-github',
    })).toBeInTheDocument();
  });

  it('copies and announces the result', async () => {
    // userEvent.setup() installs its own clipboard stub, so ours has to come
    // after it.
    const user = userEvent.setup();
    const writeText = stubClipboard(() => Promise.resolve());
    render(<CopyButton text="npm install" label="Copy command" />);

    await user.click(screen.getByRole('button'));

    expect(writeText).toHaveBeenCalledWith('npm install');
    // The confirmation is announced, not only repainted.
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Copied'));
  });

  it('tells the user what to do when the clipboard is blocked', async () => {
    const user = userEvent.setup();
    stubClipboard(() => Promise.reject(new Error("denied")));
    render(<CopyButton text="npm install" />);

    await user.click(screen.getByRole('button'));

    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent(/Ctrl\+C/));
  });
});
