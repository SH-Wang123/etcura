<script lang="ts">
  import { useBrowser } from '../stores/keys';
  import { api, localizeError } from '../lib/api';
  import { toast } from '../lib/toast';
  import { t } from '../lib/i18n';
  import type { LevelRow } from '../lib/types';

  const { status, basePath, selectedPrefix, treeRev, treeReset, pageSize, setPageSize,
    selectTreePrefix, selectKeyPath, refreshAll, detailLoading } = useBrowser();

  interface TreeChildren {
    rows: LevelRow[];
    cursor: string;
    hasMore: boolean;
  }

  interface VisibleRow {
    id: string; // dir -> "d:"+path, leaf -> "l:"+path, more -> "m:"+parent
    kind: 'dir' | 'leaf' | 'more';
    path: string; // full prefix (dir) / full key (leaf) / parent prefix (more)
    name: string; // relative display name
    depth: number;
    expanded: boolean;
    loading: boolean;
    meta: string; // leaf version hint
    index: number; // absolute row index (for virtual windowing)
  }

  let childrenOf: Record<string, TreeChildren | undefined> = {};
  let expandedSet: Record<string, true> = {};
  let loadingSet: Record<string, true> = {};
  let visible: VisibleRow[] = [];
  let selectedPath: string | null = null;
  let rootPrefix = '/';
  let scrollTop = 0;
  let viewport: HTMLDivElement | null = null;
  let viewportHeight = 600;

  const ROW_H = 24;
  const OVERSCAN = 10;

  let treeGeneration = 0;
  let lastRev = 0;


  // True while a refresh is in flight (tree levels or the selected value), so
  // the header button can show progress and refuse double clicks.
  $: busy = Object.keys(loadingSet).length > 0 || $detailLoading;

  // ---- flat windowing -----------------------------------------------------
  $: vStart = Math.max(0, Math.floor(scrollTop / ROW_H) - OVERSCAN);
  $: vEnd = Math.min(visible.length, Math.ceil((scrollTop + viewportHeight) / ROW_H) + OVERSCAN);
  $: vSlice = visible.slice(vStart, vEnd);

  function buildVisible(): void {
    const out: VisibleRow[] = [];
    const walk = (prefix: string, name: string, depth: number): void => {
      const expanded = !!expandedSet[prefix];
      out.push({
        id: 'd:' + prefix,
        kind: 'dir',
        path: prefix,
        name,
        depth,
        expanded,
        loading: !!loadingSet[prefix],
        meta: '',
        index: out.length,
      });
      const kids = childrenOf[prefix];
      if (expanded && kids) {
        for (const r of kids.rows) {
          if (r.type === 'segment') {
            walk(r.path, trimSlash(r.name), depth + 1);
          } else {
            out.push({
              id: 'l:' + r.path,
              kind: 'leaf',
              path: r.path,
              name: r.name,
              depth: depth + 1,
              expanded: false,
              loading: false,
              meta: `v${r.version}`,
              index: out.length,
            });
          }
        }
        if (kids.hasMore) {
          out.push({
            id: 'm:' + prefix,
            kind: 'more',
            path: prefix,
            name: 'More…',
            depth: depth + 1,
            expanded: false,
            loading: false,
            meta: '',
            index: out.length,
          });
        }
      }
    };
    walk(rootPrefix, rootPrefix, 0);
    visible = out;
  }

  function trimSlash(name: string): string {
    return name.replace(/\/$/, '');
  }

  async function fetchChildren(prefix: string, cursor: string, append: boolean): Promise<void> {
    const session = $status.session;
    const generation = treeGeneration;
    if (!session || loadingSet[prefix]) return;
    if (!append && childrenOf[prefix] !== undefined && cursor === '') return;
    loadingSet = { ...loadingSet, [prefix]: true as true };
    buildVisible();
    try {
      const res = await api.listLevel(session, prefix, cursor, $pageSize);
      if (generation !== treeGeneration || session !== $status.session) return;
      const prev = append ? childrenOf[prefix] : undefined;
      const baseRows = prev && append ? prev.rows : [];
      childrenOf = {
        ...childrenOf,
        [prefix]: {
          rows: [...baseRows, ...res.rows],
          cursor: res.nextCursor,
          hasMore: res.hasMore,
        },
      };
    } catch (e) {
      if (generation !== treeGeneration || session !== $status.session) return;
      const err = localizeError(e);
      if (err.code !== 'CANCELLED') {
        if (!append) childrenOf = { ...childrenOf, [prefix]: { rows: [], cursor: '', hasMore: false } };
        toast(err.message, 'error', err.detail);
      }
    } finally {
      if (generation === treeGeneration && session === $status.session) {
        const next = { ...loadingSet };
        delete next[prefix];
        loadingSet = next;
        buildVisible();
      }
    }
  }

  function toggle(path: string): void {
    if (expandedSet[path]) {
      const next = { ...expandedSet };
      delete next[path];
      expandedSet = next;
      buildVisible();
      return;
    }
    expandedSet = { ...expandedSet, [path]: true as true };
    buildVisible();
    if (childrenOf[path] === undefined) {
      void fetchChildren(path, '', false);
    }
  }

  function onRowClick(row: VisibleRow): void {
    if (row.kind === 'more') {
      void fetchChildren(row.path, childrenOf[row.path]?.cursor ?? '', true);
      return;
    }
    selectedPath = row.path;
    if (row.kind === 'dir') {
      selectTreePrefix(row.path);
      toggle(row.path);
    } else {
      void selectKeyPath(row.path);
    }
  }

  // Re-read every expanded node + root, keeping expansion and selection.
  async function refreshExpanded(): Promise<void> {
    ++treeGeneration;
    const targets = [rootPrefix, ...Object.keys(expandedSet)];
    const unique = [...new Set(targets)];
    childrenOf = {};
    loadingSet = {};
    buildVisible();
    await Promise.all(unique.map((p) => fetchChildren(p, '', false)));
  }

  async function rebuild(base: string, session: string, _reset: number): Promise<void> {
    ++treeGeneration;
    lastRev = $treeRev;
    rootPrefix = base;
    childrenOf = {};
    expandedSet = { [base]: true as true };
    loadingSet = {};
    selectedPath = null;
    scrollTop = 0;
    buildVisible();
    if (session) {
      await fetchChildren(base, '', false);
    }
  }

  $: treeSession = $status.session;
  $: void rebuild($basePath, treeSession, $treeReset);
  $: if ($treeRev !== lastRev) {
    lastRev = $treeRev;
    if (treeSession && childrenOf[rootPrefix] !== undefined) void refreshExpanded();
  }

  function onScroll(): void {
    if (viewport) scrollTop = viewport.scrollTop;
  }
</script>

<div class="tree">
  <div class="tree-header">
    <span class="title">{$t('tree.title')}</span>
    <button
      class="refresh"
      class:busy={busy}
      disabled={$status.state !== 'connected' || busy}
      title={$t('tree.tipRefresh')}
      aria-label={$t('tree.tipRefresh')}
      on:click={() => void refreshAll()}
    >
      <span class="glyph" class:spinning={busy}>⟳</span>
    </button>
    {#if busy}
      <span class="spinner" />
    {/if}
    <span class="spacer" />
    <label class="pagesize" title={$t('tree.tipPageSize')}>
      {$t('tree.page')}
      <select
        value={$pageSize}
        on:change={(e) => void setPageSize(Number((e.currentTarget as HTMLSelectElement).value))}
      >
        <option value={100}>100</option>
        <option value={200}>200</option>
        <option value={500}>500</option>
        <option value={1000}>1000</option>
      </select>
    </label>
  </div>

  {#if $status.state !== 'connected'}
    <div class="empty">{$t('tree.connectFirst')}</div>
  {:else}
    <div class="viewport" bind:this={viewport} bind:clientHeight={viewportHeight} on:scroll={onScroll}>
      <div class="spacer" style="height: {visible.length * ROW_H}px">
        {#each vSlice as row (row.id)}
          <div
            class="trow"
            class:selected={row.id === 'd:' + selectedPath || row.id === 'l:' + selectedPath}
            style="top: {row.index * ROW_H}px; height: {ROW_H}px; padding-left: {8 + row.depth * 14}px"
            role="button"
            tabindex="-1"
            on:click={() => onRowClick(row)}
          >
            {#if row.kind === 'dir'}
              {#if row.loading}
                <span class="spinner" />
              {:else}
                <button class="twist" tabindex="-1" title={row.expanded ? 'Collapse' : 'Expand'} on:click|stopPropagation={() => toggle(row.path)}>{row.expanded ? '▼' : '▶'}</button>
              {/if}
              <span class="name dir" title={row.path}>{row.name === '/' ? '/' : row.name}</span>
            {:else if row.kind === 'leaf'}
              <span class="dot" />
              <span class="name" title={row.path}>{row.name}</span>
              <span class="meta">{row.meta}</span>
            {:else}
              <span class="dot" />
              <span class="name more">{$t('tree.more')}</span>
            {/if}
          </div>
        {/each}
      </div>
    </div>
  {/if}
</div>

<style>
  .tree {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0; /* grid item: allow shrinking below content height */
    background: var(--panel);
    border-right: 1px solid var(--border);
  }
  .tree-header {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 32px;
    padding: 0 8px;
    border-bottom: 1px solid var(--border);
    background: var(--panel-alt);
    flex: none;
  }
  .title {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--dim);
  }
  .spacer {
    flex: 1;
  }
  .pagesize {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 11px;
    color: var(--dim);
  }
  .pagesize select {
    font-size: 10.5px;
    padding: 0 2px;
    height: 18px;
  }
  .empty {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--dim);
    font-size: 12px;
    padding: 20px;
    text-align: center;
  }
  .viewport {
    position: relative;
    overflow-y: auto;
    overflow-x: hidden;
    flex: 1;
    min-height: 0;
  }
  .spacer {
    position: relative;
  }
  .trow {
    position: absolute;
    left: 0;
    right: 0;
    display: flex;
    align-items: center;
    gap: 3px;
    padding-right: 8px;
    cursor: default;
    user-select: none;
    white-space: nowrap;
    border-bottom: 1px solid var(--row-border);
  }
  .trow:hover {
    background: var(--hover);
  }
  .trow.selected {
    background: var(--selected);
  }
  .twist {
    width: 18px;
    height: 20px;
    flex: none;
    border: none;
    background: none;
    padding: 0;
    color: var(--text);
    opacity: 0.85;
    font-size: 12px;
    line-height: 20px;
    text-align: center;
    cursor: pointer;
  }
  .twist:hover {
    opacity: 1;
    color: var(--accent);
  }
  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--dim);
    flex: none;
    margin: 0 4px;
  }
  .name {
    font-family: var(--mono);
    font-size: 11.5px;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }
  .name.dir {
    font-weight: 600;
  }
  .name.more {
    color: var(--accent);
    font-size: 11px;
  }
  .meta {
    margin-left: auto;
    font-size: 10px;
    color: var(--dim);
    font-family: var(--mono);
    flex: none;
  }
  .refresh {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 20px;
    flex: none;
    padding: 0;
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--panel);
    color: var(--text);
    font-size: 13px;
    line-height: 1;
    cursor: pointer;
  }
  .refresh:hover:not(:disabled) {
    background: var(--hover);
    border-color: var(--accent);
    color: var(--accent);
  }
  .refresh:disabled {
    opacity: 0.45;
    cursor: default;
  }
  .refresh .glyph {
    display: block;
  }
  .refresh .glyph.spinning {
    animation: spin 0.8s linear infinite;
  }
  .spinner {
    width: 10px;
    height: 10px;
    border: 1.5px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
    flex: none;
    display: inline-block;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
