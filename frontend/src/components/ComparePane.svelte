<script lang="ts">
  import { sessions, activeSession } from '../stores/connections';
  import { workbench, sessionLabel } from '../lib/workbench';
  import { localizeError } from '../lib/api';
  import type { model } from '../../wailsjs/go/models';
  import { t } from '../lib/i18n';
  import ValueDiff from './ValueDiff.svelte';

  const filters = ['different', 'all', 'source', 'target', 'changed', 'same'];
  let source = '';
  let target = '';
  let sourcePrefix = '/';
  let targetPrefix = '/';
  let filter = 'different';
  let info: model.CompareInfo | null = null;
  let compared: model.CompareRequest | null = null;
  $: settingsMatch = compared?.sourceSession === source && compared.targetSession === target &&
    compared.sourcePrefix === sourcePrefix && compared.targetPrefix === targetPrefix;
  let leftLabel = '';
  let rightLabel = '';
  let rows: model.CompareRow[] = [];
  let cursor = '';
  let more = '';
  let scanned = 0;
  let previous: string[] = [];
  let selected = new Set<string>();
  let detail: model.CompareDetail | null = null;
  let detailKey: string | null = null;
  let plan: model.SyncPlan | null = null;
  let permanent = new Set<string>();
  let results: model.SyncResult[] | null = null;
  let busy = false;
  let applying = false;
  let complete = false;
  let error = '';
  let generation = 0;
  let detailGeneration = 0;

  $: if ($sessions.length && !$sessions.some((item) => item.session === source)) source = $activeSession || $sessions[0].session;
  $: if (!$sessions.some((item) => item.session === target && item.session !== source)) {
    target = $sessions.find((item) => item.session !== source)?.session ?? '';
  }

  function toggle(key: string, keys: Set<string>): Set<string> {
    const next = new Set(keys);
    if (next.has(key)) next.delete(key);
    else next.add(key);
    return next;
  }

  async function start(): Promise<void> {
    if (busy || applying) return;
    const a = $sessions.find((item) => item.session === source);
    const b = $sessions.find((item) => item.session === target);
    if (!a || !b) return;
    const request = { sourceSession: source, targetSession: target, sourcePrefix, targetPrefix };
    const run = ++generation;
    busy = true;
    const old = info;
    info = null;
    complete = false;
    error = '';
    rows = [];
    results = null;
    plan = null;
    selected = new Set();
    detail = null;
    detailKey = null;
    previous = [];
    scanned = 0;
    try {
      if (old) await workbench.cancelCompare(old.id);
      const next = await workbench.startCompare(request);
      if (run !== generation) {
        await workbench.cancelCompare(next.id);
        return;
      }
      info = next;
      compared = request;
      leftLabel = `${sessionLabel(a)} · ${sourcePrefix}`;
      rightLabel = `${sessionLabel(b)} · ${targetPrefix}`;
      await page('', run);
    } catch (failure) {
      if (run === generation) error = localizeError(failure).message;
    } finally {
      if (run === generation) busy = false;
    }
  }

  async function page(next: string, run = generation): Promise<boolean> {
    const comparison = info;
    if (!comparison) return false;
    busy = true;
    error = '';
    try {
      const response = await workbench.comparePage(comparison.id, next, filter);
      if (run !== generation) return false;
      cursor = next;
      rows = response.rows;
      more = response.nextCursor;
      scanned = response.scanned;
      complete = true;
      return true;
    } catch (failure) {
      if (run === generation) error = localizeError(failure).message;
      return false;
    } finally {
      if (run === generation) busy = false;
    }
  }

  async function goNext(): Promise<void> {
    const current = cursor;
    if (await page(more)) previous = [...previous, current];
  }

  async function goBack(): Promise<void> {
    if (await page(previous[previous.length - 1])) previous = previous.slice(0, -1);
  }

  async function setFilter(): Promise<void> {
    previous = [];
    await page('');
  }

  function cancel(): void {
    ++generation;
    const comparison = info;
    if (comparison) void workbench.cancelCompare(comparison.id);
    info = null;
    busy = false;
    complete = false;
    rows = [];
    plan = null;
    detail = null;
    detailKey = null;
  }

  async function openRow(key: string): Promise<void> {
    if (!info) return;
    const id = info.id;
    const current = ++detailGeneration;
    detail = null;
    detailKey = key;
    try {
      const next = await workbench.compareDetail(id, key);
      if (info?.id === id && current === detailGeneration) detail = next;
    } catch (failure) {
      if (info?.id === id && current === detailGeneration) error = localizeError(failure).message;
    }
  }

  async function preview(): Promise<void> {
    if (!info || busy || !settingsMatch) return;
    const id = info.id;
    busy = true;
    error = '';
    try {
      const next = await workbench.previewSync(id, [...selected]);
      if (info?.id === id) {
        plan = next;
        results = null;
        permanent = new Set();
      }
    } catch (failure) {
      if (info?.id === id) error = localizeError(failure).message;
    } finally {
      if (info?.id === id) busy = false;
    }
  }

  async function apply(): Promise<void> {
    if (!plan || applying) return;
    applying = true;
    error = '';
    try {
      results = await workbench.applySync(plan.id, [...permanent]);
      selected = new Set();
    } catch (failure) {
      error = localizeError(failure).message;
    } finally {
      applying = false;
      plan = null;
    }
  }
</script>

<section class="compare">
  <header class="heading">
    <div><h1>{$t('work.compare')}</h1><p>{$t('work.compareHint')}</p></div>
    {#if info}<span class="revisions">{leftLabel} <b>r{info.sourceRevision}</b> → {rightLabel} <b>r{info.targetRevision}</b></span>{/if}
  </header>
  <form on:submit|preventDefault={() => void start()}>
    <fieldset disabled={busy || applying || !!plan}>
      <label>{$t('work.sourceConnection')}
        <select bind:value={source}>
          <option value="">{$t('work.selectConnection')}</option>
          {#each $sessions as item (item.session)}<option value={item.session}>{sessionLabel(item)}</option>{/each}
        </select>
      </label>
      <label>{$t('work.sourcePrefix')}<input class="mono" bind:value={sourcePrefix} required spellcheck="false" /></label>
      <label>{$t('work.targetConnection')}
        <select bind:value={target}>
          <option value="">{$t('work.selectConnection')}</option>
          {#each $sessions.filter((item) => item.session !== source) as item (item.session)}
            <option value={item.session}>{sessionLabel(item)}{item.readOnly ? ` · ${$t('work.readOnlyShort')}` : ''}</option>
          {/each}
        </select>
      </label>
      <label>{$t('work.targetPrefix')}<input class="mono" bind:value={targetPrefix} required spellcheck="false" /></label>
      <button class="btn primary" type="submit" disabled={!source || !target}>{complete ? $t('work.recompare') : $t('work.startCompare')}</button>
    </fieldset>
    {#if busy && !applying}<button type="button" class="btn" on:click={cancel}>{$t('confirm.cancel')}</button>{/if}
  </form>

  {#if error}<div class="error" role="alert">{error}</div>{/if}
  {#if results}
    <section class="results">
      <h2>{$t('work.results')}</h2>
      <div class="result-grid">
        {#each results as result (result.key)}
          <div title={result.message}><b class={result.state}>{$t(`work.${result.state}`)}</b> <code>{result.label || $t('work.prefixKey')}</code> {result.message}</div>
        {/each}
      </div>
    </section>
  {/if}
  {#if info}
    <div class="actions">
      <select aria-label={$t('work.filter')} bind:value={filter} disabled={busy || applying} on:change={() => void setFilter()}>
        {#each filters as item}<option value={item}>{$t(`work.${item}`)}</option>{/each}
      </select>
      <span>{$t('work.scanned', { count: scanned })}</span>
      <button class="btn btn-small" on:click={() => void goBack()} disabled={busy || !previous.length}>{$t('work.previous')}</button>
      <button class="btn btn-small" on:click={() => void goNext()} disabled={busy || !more}>{$t('work.next')}</button>
      <button class="btn primary push" disabled={busy || applying || info.targetReadOnly || !settingsMatch || !selected.size} on:click={() => void preview()}>
        {$t('work.preview')} ({selected.size})
      </button>
    </div>
    <div class="comparison">
      <div class="keylist">
        {#if !rows.length}<p>{busy ? $t('work.loading') : $t('work.noRows')}</p>{/if}
        {#each rows as row (row.key)}
          <div class="keyrow" class:chosen={row.key === detailKey}>
            <input type="checkbox" checked={selected.has(row.key)}
              disabled={info.targetReadOnly || applying || !['source','changed'].includes(row.state) || selected.size >= 100 && !selected.has(row.key)}
              aria-label={row.label || $t('work.prefixKey')} on:change={() => (selected = toggle(row.key, selected))} />
            <button on:click={() => void openRow(row.key)} title={row.label}>{row.label || $t('work.prefixKey')}</button>
            <span class={row.state}>{$t(`work.${row.state}`)}</span>
          </div>
        {/each}
      </div>
      <div class="details">
        {#if detail}
          <h2>{detail.row.label || $t('work.prefixKey')}</h2>
          {#key detail.row.key}<ValueDiff left={detail.source} right={detail.target} {leftLabel} {rightLabel} />{/key}
        {:else}<p>{$t('work.selectRow')}</p>{/if}
      </div>
    </div>
  {:else}<p class="placeholder">{busy ? $t('work.loading') : $t('work.compareEmpty')}</p>{/if}
</section>

{#if plan}
  <div class="overlay" role="presentation">
    <div class="preview" role="dialog" aria-modal="true" aria-label={$t('work.preview')}>
      <h2>{$t('work.preview')}</h2>
      <p>{leftLabel} → <strong>{rightLabel}</strong></p>
      <ul>
        {#each plan.rows as row (row.key)}
          <li>
            <code>{row.label || $t('work.prefixKey')}</code> · {$t(row.state === 'source' ? 'work.create' : 'work.update')}
            {#if row.sourceLease}
              <label><input type="checkbox" checked={permanent.has(row.key)} disabled={applying} on:change={() => (permanent = toggle(row.key, permanent))} /> {$t('work.permanent')}</label>
            {/if}
          </li>
        {/each}
      </ul>
      <footer>
        <button class="btn" disabled={applying} on:click={() => (plan = null)}>{$t('confirm.cancel')}</button>
        <button class="btn primary" disabled={applying} on:click={() => void apply()}>{applying ? $t('work.applying') : $t('work.apply')}</button>
      </footer>
    </div>
  </div>
{/if}

<style>
  .compare { height: 100%; display: flex; flex-direction: column; gap: 10px; padding: 18px; overflow: hidden; }
  .heading { display: flex; align-items: flex-end; justify-content: space-between; gap: 14px; }
  h1, h2, p { margin: 0; }
  h1 { font-size: 23px; line-height: 1.2; }
  h2 { font-size: 13px; margin-bottom: 9px; overflow-wrap: anywhere; }
  p, .revisions, .actions span { color: var(--dim); font-size: 12px; }
  .heading p { margin-top: 5px; }
  form, fieldset, .actions { display: flex; align-items: end; gap: 8px; flex-wrap: wrap; }
  fieldset { border: 0; padding: 0; margin: 0; }
  label { display: grid; gap: 4px; font-size: 11px; color: var(--dim); }
  fieldset input { width: 136px; }
  fieldset select { max-width: 180px; }
  .actions { align-items: center; padding: 8px; background: var(--panel-alt); border: 1px solid var(--border); border-radius: 6px; }
  .push { margin-left: auto; }
  .comparison { flex: 1; min-height: 0; display: grid; grid-template-columns: minmax(235px, .85fr) minmax(340px, 1.5fr); border: 1px solid var(--border); }
  .keylist, .details { overflow: auto; min-height: 0; }
  .details { border-left: 1px solid var(--border); padding: 12px; }
  .keylist > p, .placeholder { padding: 12px; }
  .keyrow { display: grid; grid-template-columns: auto minmax(0,1fr) auto; align-items: center; gap: 7px; padding: 4px 7px; border-bottom: 1px solid var(--border); }
  .keyrow.chosen { background: var(--panel-alt); }
  .keyrow button { padding: 3px 0; text-align: left; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text); background: none; border: 0; cursor: pointer; font-family: var(--mono); }
  .keyrow span, .results { font-size: 11px; }
  .changed, .conflict, .unknown { color: var(--warning); }
  .source, .success { color: var(--ok); }
  .error, .failed { color: var(--danger); }
  .results { border: 1px solid var(--border); padding: 8px; }
  .result-grid { display: grid; gap: 4px; max-height: 105px; overflow: auto; }
  .overlay { position: fixed; inset: 0; z-index: 55; background: #0007; display: grid; place-items: center; }
  .preview { background: var(--panel); border: 1px solid var(--border); border-radius: 8px; width: min(600px,90vw); padding: 16px; }
  .preview ul { max-height: 45vh; overflow: auto; padding-left: 20px; }
  .preview li { margin: 9px 0; }
  .preview label { display: flex; align-items: center; }
  .preview footer { display: flex; justify-content: flex-end; gap: 7px; }
</style>
