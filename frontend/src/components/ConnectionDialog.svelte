<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { api, localizeError } from '../lib/api';
  import { toast } from '../lib/toast';
  import { t, tr } from '../lib/i18n';
  import type { ConnectionConfig } from '../lib/types';

  export let config: ConnectionConfig;
  /** Whether a password is already stored in the Keychain for this profile. */
  export let hasPassword = false;

  const dispatch = createEventDispatcher<{ saved: ConnectionConfig; close: void }>();

  let form: ConnectionConfig;
  let endpointsText = '';
  let password = '';
  let rememberPassword = true;
  let testing = false;
  let saving = false;
  let testResult: string | null = null;
  let testError: string | null = null;

  $: init(config);

  function init(c: ConnectionConfig): void {
    form = JSON.parse(JSON.stringify(c)) as ConnectionConfig;
    endpointsText = form.endpoints.join('\n');
    password = '';
    rememberPassword = form.id === '' || hasPassword;
    testResult = null;
    testError = null;
  }

  function parsedEndpoints(): string[] {
    return endpointsText
      .split('\n')
      .map((s) => s.trim())
      .filter((s) => s.length > 0);
  }

  async function onTest(): Promise<void> {
    testing = true;
    testResult = null;
    testError = null;
    try {
      const res = await api.testConnection(
        { ...form, endpoints: parsedEndpoints() },
        password,
      );
      testResult = tr('conn.testOk', { rev: res.header, version: res.etcdVersion, endpoint: res.endpoint });
    } catch (e) {
      testError = localizeError(e).message;
    } finally {
      testing = false;
    }
  }

  async function onSave(): Promise<void> {
    saving = true;
    try {
      const saved = await api.saveConnection(
        { ...form, endpoints: parsedEndpoints() },
        password,
        rememberPassword,
      );
      toast(tr('conn.savedToast', { name: saved.name }), 'success');
      dispatch('saved', saved);
    } catch (e) {
      const err = localizeError(e);
      toast(err.message, 'error', err.detail);
    } finally {
      saving = false;
    }
  }

  function onKeydown(e: KeyboardEvent): void {
    if (e.key === 'Escape') dispatch('close');
  }

  async function pickFile(which: 'ca' | 'cert' | 'key'): Promise<void> {
    const rt = window.runtime;
    if (!rt?.Dialog?.Open) return;
    let selected: string | string[] | undefined;
    try {
      selected = await rt.Dialog.Open({
        Title: which === 'ca' ? tr('conn.ca') : which === 'cert' ? tr('conn.clientCert') : tr('conn.clientKey'),
        Filters: [{ DisplayName: 'PEM files', Pattern: '*.pem;*.crt;*.cer;*.key;*.ca' }],
      });
    } catch {
      return;
    }
    const path = Array.isArray(selected) ? selected[0] : selected;
    if (!path) return;
    if (which === 'ca') form.tls.caFile = path;
    else if (which === 'cert') form.tls.certFile = path;
    else form.tls.keyFile = path;
  }
</script>

<svelte:window on:keydown={onKeydown} />

<div class="overlay" role="presentation" on:click|self={() => dispatch('close')}>
  <div class="dialog" role="dialog" aria-modal="true">
    <div class="dialog-title">{form.id === '' ? $t('conn.addTitle') : $t('conn.editTitle', { name: config.name })}</div>

    <div class="dialog-body">
      <div class="grid">
        <label class="field span2">
          <span>{$t('conn.name')}</span>
          <input bind:value={form.name} placeholder="production-etcd" spellcheck="false" />
        </label>

        <label class="field span2">
          <span>{$t('conn.endpoints')} <em>{$t('conn.endpointsHint')}</em></span>
          <textarea
            bind:value={endpointsText}
            rows="3"
            class="mono"
            spellcheck="false"
            placeholder={$t('conn.endpointsPlaceholder')}
          />
        </label>

        <label class="field">
          <span>{$t('conn.username')} <em>{$t('conn.optional')}</em></span>
          <input bind:value={form.username} placeholder={$t('conn.usernamePlaceholder')} spellcheck="false" autocomplete="off" />
        </label>

        <label class="field">
          <span>
            {$t('conn.password')} <em>{$t('conn.optional')}{hasPassword ? $t('conn.passwordStored') : ''}</em>
          </span>
          <input
            type="password"
            bind:value={password}
            placeholder={hasPassword && password === '' ? $t('conn.passwordPlaceholderSaved') : ''}
            autocomplete="new-password"
          />
        </label>

        <label class="check span2">
          <input type="checkbox" bind:checked={rememberPassword} />
          <span>{$t('conn.remember')}</span>
        </label>

        <label class="field">
          <span>{$t('conn.dialTimeout')}</span>
          <input type="number" min="500" step="500" bind:value={form.dialTimeoutMs} />
        </label>

        <label class="field">
          <span>{$t('conn.requestTimeout')}</span>
          <input type="number" min="500" step="500" bind:value={form.requestTimeoutMs} />
        </label>

        <div class="tls-section span2">
          <label class="check">
            <input type="checkbox" bind:checked={form.tls.enabled} />
            <span><strong>{$t('conn.enableTLS')}</strong></span>
          </label>

          {#if form.tls.enabled}
            <div class="tls-grid">
              <span class="file-label">{$t('conn.ca')}</span>
              <input class="mono" bind:value={form.tls.caFile} placeholder={$t('conn.caPlaceholder')} spellcheck="false" />
              <button class="btn btn-tiny" on:click={() => void pickFile('ca')}>…</button>

              <span class="file-label">{$t('conn.clientCert')}</span>
              <input class="mono" bind:value={form.tls.certFile} placeholder={$t('conn.certPlaceholder')} spellcheck="false" />
              <button class="btn btn-tiny" on:click={() => void pickFile('cert')}>…</button>

              <span class="file-label">{$t('conn.clientKey')}</span>
              <input class="mono" bind:value={form.tls.keyFile} placeholder={$t('conn.keyPlaceholder')} spellcheck="false" />
              <button class="btn btn-tiny" on:click={() => void pickFile('key')}>…</button>

              <span class="file-label" />
              <label class="check">
                <input type="checkbox" bind:checked={form.tls.skipVerify} />
                <span>{$t('conn.skipVerify')}</span>
              </label>
              <span />
            </div>
          {/if}
        </div>
      </div>

      {#if testResult}
        <div class="test-ok">{testResult}</div>
      {:else if testError}
        <div class="test-error">{testError}</div>
      {/if}
    </div>

    <div class="dialog-footer">
      <button class="btn" on:click={() => void onTest()} disabled={testing || saving}>
        {testing ? $t('conn.testing') : $t('conn.test')}
      </button>
      <span class="spacer" />
      <button class="btn" on:click={() => dispatch('close')}>{$t('conn.cancel')}</button>
      <button class="btn primary" on:click={() => void onSave()} disabled={saving || testing}>
        {saving ? $t('conn.saving') : $t('conn.save')}
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
    padding-top: 7vh;
    z-index: 40;
  }
  .dialog {
    width: 560px;
    max-width: calc(100vw - 40px);
    max-height: 84vh;
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
    flex: none;
  }
  .dialog-body {
    padding: 12px 14px;
    overflow-y: auto;
  }
  .grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px 12px;
  }
  .span2 {
    grid-column: span 2;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }
  .field span,
  .file-label {
    font-size: 11.5px;
    color: var(--dim);
  }
  .field em {
    font-style: normal;
    color: var(--dim);
    opacity: 0.8;
  }
  .check {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
  }
  .tls-section {
    border-top: 1px dashed var(--border);
    padding-top: 10px;
  }
  .tls-grid {
    display: grid;
    grid-template-columns: 90px 1fr 30px;
    gap: 6px 8px;
    align-items: center;
    margin-top: 8px;
  }
  .tls-grid .check {
    grid-column: 2 / span 2;
  }
  .test-ok,
  .test-error {
    margin-top: 10px;
    padding: 6px 10px;
    border-radius: 4px;
    font-size: 12px;
    word-break: break-all;
  }
  .test-ok {
    background: var(--ok-bg);
    color: var(--ok);
    border: 1px solid var(--ok);
  }
  .test-error {
    background: var(--danger-bg);
    color: var(--danger);
    border: 1px solid var(--danger);
  }
  .dialog-footer {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 14px;
    border-top: 1px solid var(--border);
    flex: none;
  }
  .spacer {
    flex: 1;
  }
  .mono {
    font-family: var(--mono);
  }
</style>
