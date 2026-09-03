import { writable } from 'svelte/store';

export type ToastKind = 'info' | 'success' | 'error';

export interface Toast {
  id: number;
  kind: ToastKind;
  message: string;
  detail?: string;
}

export const toasts = writable<Toast[]>([]);

let nextID = 1;

export function toast(message: string, kind: ToastKind = 'info', detail?: string): void {
  const id = nextID++;
  toasts.update((list) => [...list.slice(-4), { id, kind, message, detail }]);
  setTimeout(() => {
    toasts.update((list) => list.filter((t) => t.id !== id));
  }, kind === 'error' ? 6000 : 3500);
}

export function dismissToast(id: number): void {
  toasts.update((list) => list.filter((t) => t.id !== id));
}
