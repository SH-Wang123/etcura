import { writable, get } from 'svelte/store';
import { api, localizeError } from '../lib/api';
import type { KvDetail, StatusInfo } from '../lib/types';
import { getContext, setContext } from 'svelte';
import { toast } from '../lib/toast';

export type SearchMode = 'prefix' | 'exact';

const browserContext = Symbol('browser');

export function provideBrowser(initial: StatusInfo) {
  const context = createBrowser(initial);
  setContext(browserContext, context);
  return context;
}

export function useBrowser(): ReturnType<typeof createBrowser> {
  return getContext(browserContext);
}

function createBrowser(initial: StatusInfo) {
  const status = writable(initial);
  /**
   * Three-pane browsing with a tree-shaped keys column:
   *  - basePath   the root scope of the tree (toolbar input)
   *  - selectedPrefix the directory currently selected (default location for
   *    creating new keys)
   * The tree itself (lazy expansion, per-node pagination) lives in KeyTree;
   * the stores below hold cross-cutting state and signals.
   */
  const basePath = writable('/');
  const selectedPrefix = writable('/');
  const searchMode = writable<SearchMode>('prefix');
  const pageSize = writable(200);

  // treeReset: rebuild the tree from the root (collapses everything).
  // treeRev:   refresh the expanded branches in place (create/delete, ⟳, ⌘R).
  const treeReset = writable(0);
  const treeRev = writable(0);
  function resetTree(): void {
    treeReset.update((n) => n + 1);
  }
  function bumpTree(): void {
    treeRev.update((n) => n + 1);
  }

  const selectedKey = writable<string | null>(null);
  const detail = writable<KvDetail | null>(null);
  const detailLoading = writable(false);

  // Incremented by the ⌘S shortcut; the inspector reacts by saving if dirty.
  const saveSignal = writable(0);
  function requestSave(): void {
    saveSignal.update((n) => n + 1);
  }

  let detailGen = 0;

  function normalizeBase(v: string): string {
    const t = v.trim();
    if (t === '') return '/';
    return t.endsWith('/') ? t : `${t}/`;
  }

  /** Toolbar input: change the tree root scope and rebuild. */
  async function setBasePath(v: string): Promise<void> {
    const normalized = normalizeBase(v);
    basePath.set(normalized);
    selectedPrefix.set(normalized);
    resetTree();
  }

  /** Directory selected in the tree (new-key default location). */
  function selectTreePrefix(p: string): void {
    selectedPrefix.set(p);
  }

  /**
   * Exact-key lookup: fetches the key straight into the inspector without any
   * listing or filtering on the frontend.
   */
  async function exactLookup(key: string): Promise<boolean> {
    const session = get(status).session;
    if (!key || !session) return false;
    const gen = ++detailGen;
    detailLoading.set(true);
    try {
      const d = await api.getKey(session, key);
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
  async function selectKeyPath(key: string): Promise<void> {
    const session = get(status).session;
    if (!session) return;
    detail.set(null);
    const gen = ++detailGen;
    selectedKey.set(key);
    detailLoading.set(true);
    try {
      const d = await api.getKey(session, key);
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
  async function reloadDetail(): Promise<void> {
    const key = get(selectedKey);
    const session = get(status).session;
    if (!key || !session) return;
    const gen = ++detailGen;
    detailLoading.set(true);
    try {
      const d = await api.getKey(session, key);
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
  function updateDetail(d: KvDetail): void {
    ++detailGen;
    detailLoading.set(false);
    detail.set(d);
    selectedKey.set(d.key);
  }

  function clearSelection(): void {
    ++detailGen;
    detailLoading.set(false);
    selectedKey.set(null);
    detail.set(null);
  }

  /** ⌘R / manual refresh: re-read the expanded branches, keep selection. */
  async function refreshCurrent(): Promise<void> {
    bumpTree();
  }

  /**
   * Full refresh of everything on screen: re-read the expanded tree levels and
   * reload the currently selected key (metadata + value), so a manual refresh
   * never leaves stale data in the inspector.
   */
  async function refreshAll(): Promise<void> {
    bumpTree();
    await reloadDetail();
  }

  async function setPageSize(n: number): Promise<void> {
    pageSize.set(n);
    bumpTree(); // re-read expanded branches with the new page size
  }

  return {
    status, basePath, selectedPrefix, searchMode, pageSize, treeRev, treeReset,
    selectedKey, detail, detailLoading, saveSignal, requestSave, setBasePath,
    selectTreePrefix, exactLookup, selectKeyPath, reloadDetail, updateDetail,
    clearSelection, refreshCurrent, refreshAll, setPageSize, bumpTree,
  };
}
