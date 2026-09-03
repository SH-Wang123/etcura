import { writable, get } from 'svelte/store';
import { api, localizeError } from '../lib/api';
import type { ConnectionSummary, StatusInfo } from '../lib/types';
import { toast } from '../lib/toast';
import { tr } from '../lib/i18n';
import { keysStore } from './keys';

export const connections = writable<ConnectionSummary[]>([]);
export const activeId = writable<string | null>(null);
export const connectingId = writable<string | null>(null);

export const status = writable<StatusInfo>({
  state: 'disconnected',
  name: '',
  endpoints: '',
  header: 0,
  etcdVersion: '',
  message: '',
});

export async function loadConnections(): Promise<void> {
  try {
    connections.set(await api.listConnections());
  } catch (e) {
    toast(localizeError(e).message, 'error');
  }
}

export async function refreshStatus(): Promise<void> {
  try {
    status.set(await api.getStatus());
  } catch {
    /* status is best effort */
  }
}

export async function connect(id: string): Promise<boolean> {
  connectingId.set(id);
  try {
    const st = await api.connect(id);
    status.set(st);
    if (st.state === 'connected') {
      activeId.set(id);
      toast(tr('connections.connectedToast', { name: st.name }), 'success');
      await keysStore.reloadAfterConnect();
      return true;
    }
    return false;
  } catch (e) {
    const err = localizeError(e);
    toast(err.message, 'error', err.detail);
    await refreshStatus();
    return false;
  } finally {
    connectingId.set(null);
  }
}

export async function disconnect(): Promise<void> {
  try {
    status.set(await api.disconnect());
    activeId.set(null);
    keysStore.reset();
  } catch (e) {
    toast(localizeError(e).message, 'error');
  }
}

/** True when the app currently has a usable etcd connection. */
export function isConnected(): boolean {
  return get(status).state === 'connected';
}
