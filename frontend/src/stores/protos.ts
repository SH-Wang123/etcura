import { writable } from 'svelte/store';
import { api, localizeError } from '../lib/api';
import type { ProtoFileInfo } from '../lib/types';
import { toast } from '../lib/toast';
import { tr } from '../lib/i18n';

export const importedProtos = writable<ProtoFileInfo[]>([]);

export async function loadProtos(): Promise<void> {
  try {
    importedProtos.set(await api.listProtos());
  } catch {
    /* best effort (no runtime in plain browser) */
  }
}

export async function importProtos(paths: string[]): Promise<boolean> {
  try {
    const files = await api.importProtos(paths);
    importedProtos.set(files);
    const messages = files.reduce((a, f) => a + f.messageCount, 0);
    toast(tr('proto.importedToast', { n: files.length, m: messages }), 'success');
    return true;
  } catch (e) {
    const err = localizeError(e);
    toast(err.message, 'error', err.detail);
    return false;
  }
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
