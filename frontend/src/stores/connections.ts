import { writable, get } from 'svelte/store';
import { api, localizeError } from '../lib/api';
import type { ConnectionSummary, StatusInfo } from '../lib/types';
import { toast } from '../lib/toast';

export const connections = writable<ConnectionSummary[]>([]);
export const sessions = writable<StatusInfo[]>([]);
export const activeSession = writable('');
export const connecting = writable<string[]>([]);
let request = 0;

export async function loadConnections(): Promise<void> {
  try {
    connections.set(await api.listConnections() ?? []);
  } catch (error) {
    toast(localizeError(error).message, 'error');
  }
}

export async function refreshStatus(): Promise<void> {
  const generation = ++request;
  try {
    const result = await api.listSessions();
    if (generation !== request) return;
    const open = new Map(result.map((session) => [session.session, session]));
    const previous = get(sessions);
    const next = previous.flatMap((session) => open.has(session.session) ? [open.get(session.session)!] : []);
    const known = new Set(next.map((session) => session.session));
    next.push(...result.filter((session) => !known.has(session.session)));
    sessions.set(next);
    if (!open.has(get(activeSession))) activeSession.set(next[0]?.session ?? '');
  } catch (error) {
    toast(localizeError(error).message, 'error');
  }
}

export async function connect(id: string): Promise<void> {
  const open = get(sessions).find((session) => session.profileId === id);
  if (open) {
    activeSession.set(open.session);
    return;
  }
  if (get(connecting).includes(id)) return;
  connecting.update((ids) => [...ids, id]);
  try {
    const session = await api.connect(id);
    ++request;
    sessions.update((all) => [...all.filter((item) => item.profileId !== id), session]);
    activeSession.set(session.session);
  } catch (error) {
    const failure = localizeError(error);
    toast(failure.message, 'error', failure.detail);
  } finally {
    connecting.update((ids) => ids.filter((item) => item !== id));
  }
}

export async function disconnect(session: string): Promise<void> {
  try {
    await api.disconnect(session);
    ++request;
    sessions.update((items) => items.filter((item) => item.session !== session));
    if (get(activeSession) === session) activeSession.set(get(sessions)[0]?.session ?? '');
  } catch (error) {
    toast(localizeError(error).message, 'error');
  }
}
