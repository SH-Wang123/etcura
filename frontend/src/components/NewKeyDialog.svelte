<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { api, localizeError } from '../lib/api';
  import { askConfirm } from '../lib/confirm';
  import { toast } from '../lib/toast';
  import { t, tr } from '../lib/i18n';
  import { utf8Encode, bytesToB64 } from '../lib/base64';
  import type { KvDetail } from '../lib/types';

  export let defaultPrefix = '/';

  const dispatch = createEventDispatcher<{ created: KvDetail; close: void }>();

  let key = defaultPrefix;
  let value = '';
  let leaseID = '';
  let creating = false;

  async function onCreate(): Promise<void> {
    const k = key.trim();
    if (!k) {
      toast(tr('newkey.emptyKey'), 'error');
      return;
    }
    creating = true;
    try {
      const b64 = bytesToB64(utf8Encode(value));
      const created = await api.createKey(k, b64, leaseID.trim());
      toast(tr('newkey.createdToast', { key: k }), 'success');
      dispatch('created', created);
      return;
    } catch (e) {
      const err = localizeError(e);
      if (err.code === 'KEY_ALREADY_EXISTS') {
        const ok = await askConfirm({
          title: tr('newkey.existsTitle'),
          lines: [tr('newkey.existsKeyLine', { key: k }), '', tr('newkey.existsQuestion')],
          confirmLabel: tr('newkey.overwrite'),
        });
        if (ok) await overwrite(k);
      } else {
        toast(err.message, 'error', err.detail);
      }
    } finally {
      creating = false;
    }
  }

  // Overwrite path: load the current record, then save under a CAS check so a
  // concurrent writer between the two calls still surfaces a conflict.
  async function overwrite(k: string): Promise<void> {
    try {
      const existing = await api.getKey(k);
      const saved = await api.saveKey(k, bytesToB64(utf8Encode(value)), existing.modRevision);
      toast(tr('newkey.overwroteToast', { key: k }), 'success');
      dispatch('created', saved);
    } catch (e) {
      const err = localizeError(e);
      toast(err.message, 'error', err.detail);
    }
  }

  function onKeydown(e: KeyboardEvent): void {
    if (e.key === 'Escape') dispatch('close');
  }
</script>

<svelte:window on:keydown={onKeydown} />

<div class="overlay" role="presentation" on:click|self={() => dispatch('close')}>
  <div class="dialog" role="dialog" aria-modal="true">
    <div class="dialog-title">{$t('newkey.title')}</div>
    <div class="dialog-body">
      <label class="field">
        <span>{$t('newkey.key')}</span>
        <input class="mono" bind:value={key} placeholder={$t('newkey.keyPlaceholder')} spellcheck="false" />
      </label>
      <label class="field">
        <span>{$t('newkey.value')}</span>
        <textarea class="mono" bind:value={value} rows="8" spellcheck="false" placeholder={$t('newkey.valuePlaceholder')} />
      </label>
      <label class="field narrow">
        <span>{$t('newkey.lease')} <em>{$t('conn.optional')}</em></span>
        <input class="mono" bind:value={leaseID} placeholder="7587839385437545011" spellcheck="false" />
      </label>
      <p class="hint">{$t('newkey.hint')}</p>
    </div>
    <div class="dialog-footer">
      <span class="spacer" />
      <button class="btn" on:click={() => dispatch('close')}>{$t('conn.cancel')}</button>
      <button class="btn primary" on:click={() => void onCreate()} disabled={creating}>
        {creating ? $t('newkey.creating') : $t('newkey.create')}
      </button>
    </div>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.35);
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding-top: 10vh;
    z-index: 40;
  }
  .dialog {
    width: 540px;
    max-width: calc(100vw - 40px);
    display: flex;
    flex-direction: column;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    box-shadow: 0 18px 50px rgba(0, 0, 0, 0.35);
  }
  .dialog-title {
    padding: 10px 14px;
    font-size: 13px;
    font-weight: 600;
    border-bottom: 1px solid var(--border);
  }
  .dialog-body {
    padding: 12px 14px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .field.narrow {
    max-width: 320px;
  }
  .field span {
    font-size: 11.5px;
    color: var(--dim);
  }
  .field em {
    font-style: normal;
    opacity: 0.8;
  }
  .hint {
    margin: 0;
    font-size: 11.5px;
    color: var(--dim);
  }
  .dialog-footer {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 14px;
    border-top: 1px solid var(--border);
  }
  .spacer {
    flex: 1;
  }
  .mono {
    font-family: var(--mono);
  }
</style>
