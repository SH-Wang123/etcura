import { writable, get } from 'svelte/store';
import { api, localizeError } from '../lib/api';
import type { KvDetail } from '../lib/types';
import { toast } from '../lib/toast';

export type SearchMode = 'prefix' | 'exact';

/**
 * Three-pane browsing with a tree-shaped keys column:
 *  - basePath   the root scope of the tree (toolbar input)
 *  - selectedPrefix the directory currently selected (default location for
 *    creating new keys)
 * The tree itself (lazy expansion, per-node pagination) lives in KeyTree;
 * the stores below hold cross-cutting state and signals.
 */
export const basePath = writable('/');
export const selectedPrefix = writable('/');
export const searchMode = writable<SearchMode>('prefix');
export const pageSize = writable(200);

// treeReset: rebuild the tree from the root (collapses everything).
// treeRev:   refresh the expanded branches in place (create/delete, ⟳, ⌘R).
export const treeReset = writable(0);
export const treeRev = writable(0);
export function resetTree(): void {
  treeReset.update((n) => n + 1);
}
export function bumpTree(): void {
  treeRev.update((n) => n + 1);
}

export const selectedKey = writable<string | null>(null);
export const detail = writable<KvDetail | null>(null);
export const detailLoading = writable(false);

// Incremented by the ⌘S shortcut; the inspector reacts by saving if dirty.
export const saveSignal = writable(0);
export function requestSave(): void {
  saveSignal.update((n) => n + 1);
}

let detailGen = 0;

function normalizeBase(v: string): string {
  const t = v.trim();
  if (t === '') return '/';
  return t.endsWith('/') ? t : `${t}/`;
}

/** Toolbar input: change the tree root scope and rebuild. */
export async function setBasePath(v: string): Promise<void> {
  const normalized = normalizeBase(v);
  basePath.set(normalized);
  selectedPrefix.set(normalized);
  resetTree();
}

/** Directory selected in the tree (new-key default location). */
export function selectTreePrefix(p: string): void {
  selectedPrefix.set(p);
}

/**
 * Exact-key lookup: fetches the key straight into the inspector without any
 * listing or filtering on the frontend.
 */
export async function exactLookup(key: string): Promise<boolean> {
  const gen = ++detailGen;
  const trimmed = key.trim();
  if (!trimmed) return false;
  detailLoading.set(true);
  try {
    const d = await api.getKey(trimmed);
    if (gen !== detailGen) return false;
    selectedKey.set(d.key);
    detail.set(d);
    return true;
  } catch (e) {
    if (gen === detailGen) {
      const err = localizeError(e);
      toast(err.message, 'error', err.detail);
      detail.set(null);
      selectedKey.set(null);
    }
    return false;
  } finally {
    if (gen === detailGen) detailLoading.set(false);
  }
}

/** Load a key into the inspector by path (tree leaf selection). */
export async function selectKeyPath(key: string): Promise<void> {
  const gen = ++detailGen;
  selectedKey.set(key);
  detailLoading.set(true);
  try {
    const d = await api.getKey(key);
    if (gen !== detailGen || get(selectedKey) !== key) return;
    detail.set(d);
  } catch (e) {
    if (gen !== detailGen) return;
    const err = localizeError(e);
    toast(err.message, 'error', err.detail);
    detail.set(null);
  } finally {
    if (gen === detailGen) detailLoading.set(false);
  }
}

/** Reload the selected key (Refresh button in the inspector). */
export async function reloadDetail(): Promise<void> {
  const key = get(selectedKey);
  if (!key) return;
  const gen = ++detailGen;
  detailLoading.set(true);
  try {
    const d = await api.getKey(key);
    if (gen !== detailGen || get(selectedKey) !== key) return;
    detail.set(d);
  } catch (e) {
    if (gen === detailGen) {
      const err = localizeError(e);
      toast(err.message, 'error', err.detail);
    }
  } finally {
    if (gen === detailGen) detailLoading.set(false);
  }
}

/** Replace the stored detail after a successful save/create. */
export function updateDetail(d: KvDetail): void {
  detail.set(d);
  selectedKey.set(d.key);
}

export function clearSelection(): void {
  ++detailGen;
  selectedKey.set(null);
  detail.set(null);
}

/** ⌘R / manual refresh: re-read the expanded branches, keep selection. */
export async function refreshCurrent(): Promise<void> {
  bumpTree();
}

/**
 * Full refresh of everything on screen: re-read the expanded tree levels and
 * reload the currently selected key (metadata + value), so a manual refresh
 * never leaves stale data in the inspector.
 */
export async function refreshAll(): Promise<void> {
  bumpTree();
  await reloadDetail();
}

/** Rebuild the tree from the root (connect, base change). */
export async function loadFirst(): Promise<void> {
  clearSelection();
  resetTree();
}

export async function setPageSize(n: number): Promise<void> {
  pageSize.set(n);
  bumpTree(); // re-read expanded branches with the new page size
}

/** Clear everything (disconnect). */
export function reset(): void {
  ++detailGen;
  selectedKey.set(null);
  detail.set(null);
}

/** After connecting, build the tree for the current base. */
export async function reloadAfterConnect(): Promise<void> {
  clearSelection();
  resetTree();
}

export const keysStore = {
  setBasePath,
  selectTreePrefix,
  exactLookup,
  selectKeyPath,
  reloadDetail,
  updateDetail,
  clearSelection,
  reset,
  reloadAfterConnect,
};
