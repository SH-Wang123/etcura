<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { scriptInfo, saveScript, clearScript, emptyScriptConfig } from '../stores/script';
  import { t } from '../lib/i18n';
  import type { ScriptConfig, ScriptLanguage } from '../lib/types';

  const dispatch = createEventDispatcher<{ close: void }>();

  const languages: { id: ScriptLanguage; label: string }[] = [
    { id: 'python', label: 'Python 3' },
    { id: 'node', label: 'Node.js' },
    { id: 'ruby', label: 'Ruby' },
    { id: 'shell', label: 'Shell (sh)' },
  ];

  let form: ScriptConfig = $scriptInfo
    ? { ...emptyScriptConfig(), language: $scriptInfo.language, inputVar: $scriptInfo.inputVar, outputVar: $scriptInfo.outputVar, body: '' }
    : emptyScriptConfig();
  let busy = false;
  let loadError = '';

  // The body is not part of ScriptInfo (it can be large), so prefill it from a
  // template when configuring for the first time.
  $: if (form.body === '' && !$scriptInfo) {
    form.body = templateFor(form.language);
  }

  function templateFor(lang: ScriptLanguage): string {
    switch (lang) {
      case 'python':
        return 'result = payload.hex().upper()';
      case 'node':
        return "const result = payload.toString('hex').toUpperCase();";
      case 'ruby':
        return "result = payload.unpack1('H*').upcase";
      case 'shell':
        return 'result=$(od -An -tx1 < "$payload" | tr -d " \\n")';
    }
  }

  function onLanguageChange(e: Event): void {
    const lang = (e.currentTarget as HTMLSelectElement).value as ScriptLanguage;
    // Swap the starter body when it is still the previous language's template.
    const previous = languages.map((l) => templateFor(l.id));
    form.language = lang;
    if (form.body.trim() === '' || previous.includes(form.body)) {
      form.body = templateFor(lang);
    }
  }

  async function apply(): Promise<void> {
    busy = true;
    const ok = await saveScript({ ...form, body: form.body });
    busy = false;
    if (ok) dispatch('close');
  }

  async function remove(): Promise<void> {
    busy = true;
    await clearScript();
    busy = false;
    dispatch('close');
  }

  function onKeydown(e: KeyboardEvent): void {
    if (e.key === 'Escape') dispatch('close');
  }
</script>

<svelte:window on:keydown={onKeydown} />

<div class="overlay" role="presentation" on:click|self={() => dispatch('close')}>
  <div class="dialog" role="dialog" aria-modal="true">
    <div class="dialog-title">{$t('script.title')}</div>

    <div class="dialog-body">
      <p class="hint">{$t('script.hint')}</p>

      <div class="grid">
        <label class="field">
          <span>{$t('script.language')}</span>
          <select value={form.language} on:change={onLanguageChange}>
            {#each languages as l (l.id)}
              <option value={l.id}>{l.label}</option>
            {/each}
          </select>
        </label>
        <label class="field">
          <span>{$t('script.inputVar')}</span>
          <input class="mono" bind:value={form.inputVar} placeholder={$t('script.varOptional')} spellcheck="false" />
        </label>
        <label class="field">
          <span>{$t('script.outputVar')}</span>
          <input class="mono" bind:value={form.outputVar} placeholder={$t('script.varOptional')} spellcheck="false" />
        </label>
      </div>

      <p class="binding-note">
        {#if form.inputVar}
          {$t('script.inputNote', { v: form.inputVar })}
        {:else}
          {$t('script.inputNoteEmpty')}
        {/if}
        {#if form.outputVar}
          {$t('script.outputNote', { v: form.outputVar })}
        {:else}
          {$t('script.outputNoteEmpty')}
        {/if}
      </p>

      <label class="field body-field">
        <span>{$t('script.body')}</span>
        <textarea class="mono" rows="10" bind:value={form.body} spellcheck="false" />
      </label>

      {#if $scriptInfo?.warning}
        <div class="warn">{$scriptInfo.warning}</div>
      {/if}
      {#if loadError}
        <div class="warn">{loadError}</div>
      {/if}
    </div>

    <div class="dialog-footer">
      {#if $scriptInfo}
        <button class="btn danger" on:click={() => void remove()} disabled={busy}>{$t('script.clear')}</button>
      {/if}
      <span class="spacer" />
      <button class="btn" on:click={() => dispatch('close')}>{$t('script.close')}</button>
      <button class="btn primary" on:click={() => void apply()} disabled={busy || form.body.trim() === ''}>
        {busy ? $t('script.applying') : $t('script.apply')}
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
    width: 640px;
    max-width: calc(100vw - 40px);
    max-height: 86vh;
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
    overflow-y: auto;
  }
  .hint {
    margin: 0 0 10px;
    font-size: 11.5px;
    color: var(--dim);
    line-height: 1.55;
  }
  .grid {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: 10px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }
  .field > span {
    font-size: 11.5px;
    color: var(--dim);
  }
  .body-field {
    margin-top: 10px;
  }
  .body-field textarea {
    font-size: 11.5px;
    line-height: 1.5;
    resize: vertical;
  }
  .binding-note {
    margin: 8px 0 0;
    font-size: 11px;
    color: var(--dim);
    line-height: 1.6;
  }
  .warn {
    margin-top: 8px;
    font-size: 11px;
    color: var(--danger);
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
