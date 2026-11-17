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
  profileId: '',
  session: '',
  readOnly: false,
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

let statusRequest = 0;
let disconnecting = false;

function applyStatus(next: StatusInfo): void {
  if (next.session !== get(status).session) keysStore.reset();
  activeId.set(next.profileId || null);
  status.set(next);
}

export async function refreshStatus(): Promise<void> {
  const request = ++statusRequest;
  try {
    const next = await api.getStatus();
    if (request === statusRequest) applyStatus(next);
  } catch {
    /* status is best effort */
  }
}

export async function connect(id: string): Promise<boolean> {
  if (get(connectingId) || disconnecting) return false;
  connectingId.set(id);
  ++statusRequest;
  applyStatus({ ...get(status), state: 'connecting', profileId: '', session: '', readOnly: false });
  try {
    const next = await api.connect(id);
    ++statusRequest;
    applyStatus(next);
    if (next.state !== 'connected') return false;
    toast(tr('connections.connectedToast', { name: next.name }), 'success');
    return true;
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
  if (get(connectingId) || disconnecting) return;
  disconnecting = true;
  ++statusRequest;
  try {
    const next = await api.disconnect();
    ++statusRequest;
    applyStatus(next);
  } catch (e) {
    toast(localizeError(e).message, 'error');
  } finally {
    disconnecting = false;
  }
}

/** True when the app currently has a usable etcd connection. */
export function isConnected(): boolean {
  return get(status).state === 'connected';
}
