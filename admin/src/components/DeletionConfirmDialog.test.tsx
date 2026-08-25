import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import DeletionConfirmDialog from './DeletionConfirmDialog';

describe('DeletionConfirmDialog', () => {
  it('requires typed confirmation and acknowledgement before enabling the destructive action', async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();

    render(
      <DeletionConfirmDialog
        open
        title="Delete plugin?"
        message="This removes the plugin."
        confirmationValue="@semrel/provider-github"
        confirmationLabel="Plugin name"
        confirmLabel="Delete plugin"
        onClose={() => undefined}
        onConfirm={onConfirm}
      />,
    );

    const confirmButton = screen.getByRole('button', { name: 'Delete plugin' });
    expect(confirmButton).toBeDisabled();

    await user.type(screen.getByLabelText('Plugin name'), '@semrel/provider-github');
    expect(confirmButton).toBeDisabled();

    await user.click(screen.getByLabelText('I understand this action cannot be undone.'));
    expect(confirmButton).toBeEnabled();

    await user.click(confirmButton);
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });
});
