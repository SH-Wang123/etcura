import { writable, get } from 'svelte/store';

export interface ConfirmOptions {
  title: string;
  /** Lines shown in the body; rendered pre-formatted so blank lines work. */
  lines: string[];
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
}

interface ConfirmRequest extends ConfirmOptions {
  resolve: (ok: boolean) => void;
}

export const confirmState = writable<ConfirmRequest | null>(null);

/** Opens the modal confirm dialog and resolves with the user's answer. */
export function askConfirm(opts: ConfirmOptions): Promise<boolean> {
  return new Promise((resolve) => confirmState.set({ ...opts, resolve }));
}

/** Called by ConfirmDialog when the user clicks a button. */
export function answerConfirm(ok: boolean): void {
  const req = get(confirmState);
  confirmState.set(null);
  req?.resolve(ok);
}
