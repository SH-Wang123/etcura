<script lang="ts">
  import { createEventDispatcher, onDestroy } from 'svelte';
  import { useBrowser } from '../stores/keys';
  import { t, locale, setLocale, locales } from '../lib/i18n';

  export let searchEl: HTMLInputElement | null = null;

  /** Number of imported proto message types (badge on the Protos button). */
  export let protoCount = 0;

  /** Active post-processing script label ("" when none). */
  export let scriptName = '';

  const dispatch = createEventDispatcher<{ newkey: void; protos: void; script: void }>();

  const { status, basePath, searchMode, exactLookup, refreshAll, setBasePath } = useBrowser();
  let search = $basePath;
  let baseTimer: ReturnType<typeof setTimeout> | null = null;
  onDestroy(() => { if (baseTimer) clearTimeout(baseTimer); });

  function onModeChange(e: Event): void {
    searchMode.set((e.currentTarget as HTMLSelectElement).value as 'prefix' | 'exact');
  }

  // Debounced base-prefix input: after the user stops typing, the tree
  // switches to the new scope (the backend cancels the previous request).
  function onSearchInput(): void {
    if ($searchMode !== 'prefix') return;
    if (baseTimer) clearTimeout(baseTimer);
    baseTimer = setTimeout(() => {
      void setBasePath(search);
    }, 300);
  }

  function onSearchKeydown(e: KeyboardEvent): void {
    if (e.key !== 'Enter') return;
    if (baseTimer) clearTimeout(baseTimer);
    if ($searchMode === 'exact') void exactLookup(search);
    else void setBasePath(search);
  }

  function onRefresh(): void {
    if ($status.session) void refreshAll();
  }

  function onNewKey(): void {
    if ($status.session) dispatch('newkey');
  }

  function onLocaleChange(e: Event): void {
    setLocale((e.currentTarget as HTMLSelectElement).value as typeof $locale);
  }
</script>

<div class="toolbar">
  <div class="search">
    <select class="mode" value={$searchMode} on:change={onModeChange} title={$t('tip.searchMode')}>
      <option value="prefix">{$t('search.prefix')}</option>
      <option value="exact">{$t('search.exact')}</option>
    </select>
    <input
      bind:this={searchEl}
      bind:value={search}
      on:input={onSearchInput}
      on:keydown={onSearchKeydown}
      placeholder={$searchMode === 'prefix' ? $t('placeholder.prefix') : $t('placeholder.exact')}
      spellcheck="false"
      autocomplete="off"
    />
  </div>

  <div class="actions">
    <button class="btn" on:click={() => dispatch('protos')} title={$t('tip.protos')}>
      {$t('btn.protos')}{protoCount > 0 ? ` (${protoCount})` : ''}
    </button>
    <button
      class="btn"
      class:active={scriptName !== ''}
      on:click={() => dispatch('script')}
      title={$t('tip.script')}
    >
      {$t('btn.script')}{scriptName !== '' ? ` · ${scriptName}` : ''}
    </button>
    <button class="btn" on:click={onNewKey} disabled={$status.state !== 'connected' || $status.readOnly} title={$t('tip.newKey')}>
      {$t('btn.newKey')}
    </button>
    <button class="btn" on:click={onRefresh} title={$t('tip.refresh')}>{$t('btn.refresh')}</button>
    <select
      class="locale"
      value={$locale}
      on:change={onLocaleChange}
      title={$t('locale.label')}
      aria-label={$t('locale.label')}
    >
      {#each locales as l (l.id)}
        <option value={l.id}>{l.label}</option>
      {/each}
    </select>
  </div>
</div>

<style>
  .toolbar {
    grid-area: toolbar;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 0 10px;
    border-bottom: 1px solid var(--border);
    background: var(--panel-alt);
  }
  .search {
    display: flex;
    align-items: center;
    gap: 6px;
    flex: 1;
    min-width: 0;
  }
  .search input {
    flex: 1;
  }
  .mode {
    flex: none;
  }
  .actions {
    flex: none;
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .locale {
    font-size: 11px;
    padding: 2px 4px;
    height: 24px;
  }
</style>
