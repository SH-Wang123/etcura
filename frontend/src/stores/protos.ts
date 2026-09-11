import { writable } from 'svelte/store';
import { api, localizeError } from '../lib/api';
import type { ProtoFileInfo } from '../lib/types';
import { toast } from '../lib/toast';
import { tr } from '../lib/i18n';

export const importedProtos = writable<ProtoFileInfo[]>([]);

/** Last import failure, shown inline in the dialog so long compiler output
 *  (with file:line) stays readable instead of vanishing with the toast. */
export const protoImportError = writable<{ message: string; detail?: string } | null>(null);

export async function loadProtos(): Promise<void> {
  try {
    importedProtos.set(await api.listProtos());
  } catch {
    /* best effort (no runtime in plain browser) */
  }
}

export async function importProtos(paths: string[]): Promise<boolean> {
  protoImportError.set(null);
  try {
    const files = await api.importProtos(paths);
    importedProtos.set(files);
    const messages = files.reduce((a, f) => a + f.messageCount, 0);
    toast(tr('proto.importedToast', { n: files.length, m: messages }), 'success');
    return true;
  } catch (e) {
    const err = localizeError(e);
    // Keep the full compiler output (file:line + selected-file mapping) in the
    // dialog; the toast would truncate it.
    protoImportError.set({ message: err.message, detail: err.detail });
    toast(err.message, 'error');
    return false;
  }
}

export function clearProtoImportError(): void {
  protoImportError.set(null);
}

export async function clearProtos(): Promise<void> {
  try {
    api.clearProtos();
    importedProtos.set([]);
    toast(tr('proto.clearedToast'), 'success');
  } catch (e) {
    toast(localizeError(e).message, 'error');
  }
}
