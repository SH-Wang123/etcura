<script lang="ts">
  import { onDestroy } from 'svelte';
  import { sessions, activeSession } from '../stores/connections';
  import { localizeError } from '../lib/api';
  import { workbench, sessionLabel } from '../lib/workbench';
  import { t } from '../lib/i18n';
  import type { model } from '../../wailsjs/go/models';
  import ValueDiff from './ValueDiff.svelte';

  const historyLimit = 128;
  let session = '';
  let key = '/';
  let scope = 'prefix';
  let watchId = '';
  let label = '';
  let busy = false;
  let state = '';
  let revision = '';
  let missed = 0;
  let error = '';
  let timer: ReturnType<typeof setTimeout> | undefined;
  let events: model.WatchEvent[] = [];
  let selected: model.WatchEvent | null = null;
  let destroyed = false;

  $: if (!watchId && !$sessions.some((item) => item.session === session)) session = $activeSession || $sessions[0]?.session || '';

  async function start(): Promise<void> {
    if (watchId || busy || !session || !key) return;
    busy = true;
    error = '';
    events = [];
    selected = null;
    missed = 0;
    revision = '';
    state = '';
    const connection = $sessions.find((item) => item.session === session);
    label = connection ? `${sessionLabel(connection)} · ${key}` : key;
    try {
      const id = await workbench.startWatch({ session, key, prefix: scope === 'prefix' });
      if (destroyed) {
        await workbench.stopWatch(id);
        return;
      }
      watchId = id;
      await poll(id);
    } catch (failure) {
      error = localizeError(failure).message;
    } finally {
      busy = false;
    }
  }

  async function poll(id: string): Promise<void> {
    try {
      const batch = await workbench.pollWatch(id);
      if (watchId !== id) return;
      state = batch.state;
      revision = batch.revision;
      if (batch.error) error = localizeError(batch.error).message;
      else error = '';
      const next = [...batch.events.reverse(), ...events];
      missed += batch.missed + Math.max(0, next.length - historyLimit);
      events = next.slice(0, historyLimit);
      if (state === 'stopped' || state === 'error') {
        await stop(false);
        return;
      }
      timer = setTimeout(() => void poll(id), 500);
    } catch (failure) {
      if (watchId !== id) return;
      state = 'error';
      error = localizeError(failure).message;
      await stop(false);
    }
  }

  async function stop(manual = true): Promise<void> {
    if (timer) clearTimeout(timer);
    const id = watchId;
    watchId = '';
    if (manual) state = 'stopped';
    if (id) {
      try { await workbench.stopWatch(id); }
      catch (failure) { error = localizeError(failure).message; }
    }
  }

  onDestroy(() => {
    destroyed = true;
    void stop(false);
  });
</script>

<section class="watch">
  <header><h1>{$t('work.watch')}</h1><p>{$t('work.watchHint')}</p></header>
  <form on:submit|preventDefault={() => void start()}>
    <fieldset disabled={busy || !!watchId}>
      <label>{$t('tip.connection')}
        <select bind:value={session}>
          <option value="">{$t('work.selectConnection')}</option>
          {#each $sessions as item (item.session)}<option value={item.session}>{sessionLabel(item)}</option>{/each}
        </select>
      </label>
      <label>{$t('work.scope')}
        <select bind:value={scope}>
          <option value="prefix">{$t('search.prefix')}</option>
          <option value="exact">{$t('search.exact')}</option>
        </select>
      </label>
      <label>{$t('work.keyPrefix')}<input class="mono" bind:value={key} required spellcheck="false" /></label>
      <button class="btn primary" type="submit" disabled={!session}>{$t('work.startWatch')}</button>
    </fieldset>
    {#if watchId}<button class="btn" type="button" on:click={() => void stop()}>{$t('work.stopWatch')}</button>{/if}
  </form>
  {#if state}
    <div class="status" role="status"><strong class={state}>{$t(`work.watch.${state}`)}</strong><span>{label}</span><code>r{revision}</code></div>
  {/if}
  {#if missed}<p class="warning">{$t('work.missed', { count: missed })}</p>{/if}
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <div class="timeline">
    <div class="events">
      {#if !events.length}<p>{$t('work.noEvents')}</p>{/if}
      {#each events as event}
        <button class="event" class:current={event === selected} on:click={() => (selected = event)}>
          <time>{new Date(event.seenAt).toLocaleTimeString()}</time>
          <b class={event.type}>{$t(`work.${event.type}`)}</b>
          <code>{event.key || $t('work.gap')}</code><small>r{event.revision}</small>
        </button>
      {/each}
    </div>
    <div class="item">
      {#if selected?.type === 'gap'}<p class="warning">{$t('work.gapDetails')}</p>
      {:else if selected}
        <h2>{selected.key}</h2>
        {#key selected}<ValueDiff left={selected.before} right={selected.after} leftLabel={$t('work.before')} rightLabel={$t('work.after')} />{/key}
      {:else}<p>{$t('work.selectEvent')}</p>{/if}
    </div>
  </div>
</section>

<style>
  .watch { height: 100%; display: flex; flex-direction: column; padding: 18px; gap: 12px; }
  h1, h2, p { margin: 0; }
  h1 { font-size: 23px; line-height: 1.2; }
  h2 { font-size: 13px; margin-bottom: 9px; overflow-wrap: anywhere; }
  p { font-size: 12px; color: var(--dim); }
  header p { margin-top: 5px; }
  form, fieldset { display: flex; flex-wrap: wrap; align-items: end; gap: 8px; }
  fieldset { border: 0; padding: 0; margin: 0; }
  label { display: grid; gap: 4px; font-size: 11px; color: var(--dim); }
  .status { display: flex; align-items: center; gap: 10px; padding: 8px; border: 1px solid var(--border); background: var(--panel-alt); font-size: 12px; }
  .timeline { flex: 1; min-height: 0; display: grid; grid-template-columns: minmax(238px, 1fr) minmax(330px, 1.35fr); border: 1px solid var(--border); }
  .events, .item { overflow: auto; }
  .item { border-left: 1px solid var(--border); padding: 12px; }
  .events p { padding: 12px; }
  .event { display: grid; grid-template-columns: auto auto minmax(0,1fr) auto; gap: 6px; align-items: center; width: 100%; text-align: left; cursor: pointer; padding: 7px; background: var(--panel); color: var(--text); border: 0; border-bottom: 1px solid var(--border); font-size: 11px; }
  .event.current { background: var(--panel-alt); }
  .event code { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  small, time { color: var(--dim); }
  .live, .create { color: var(--ok); }
  .warning, .update, .gap, .reconnecting { color: var(--warning); }
  .error, .delete { color: var(--danger); }
</style>
