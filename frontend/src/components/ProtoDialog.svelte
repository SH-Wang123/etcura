<script lang="ts">
  import { importedProtos, importProtos, clearProtos, protoImportError, clearProtoImportError } from '../stores/protos';
  import { createEventDispatcher } from 'svelte';
  import { t, tr } from '../lib/i18n';

  const dispatch = createEventDispatcher<{ close: void }>();

  let pathsText = '';
  let importing = false;
  let hasNativeDialog = !!window.runtime?.Dialog?.Open;

  function appendPaths(arr: string[]): void {
    if (arr.length === 0) return;
    pathsText = pathsText
      .split('\n')
      .map((s) => s.trim())
      .filter(Boolean)
      .concat(arr)
      .join('\n');
  }

  async function browseFiles(): Promise<void> {
    const rt = window.runtime;
    if (!rt?.Dialog?.Open) return;
    try {
      const selected = await rt.Dialog.Open({
        Title: tr('proto.selectTitle'),
        Filters: [{ DisplayName: 'Proto files', Pattern: '*.proto' }],
        MultipleAllowed: true,
      });
      appendPaths(Array.isArray(selected) ? selected : selected ? [selected] : []);
    } catch {
      /* dialog cancelled */
    }
  }

  // Picking the project root imports every .proto below it, so relative
  // imports resolve without hand-selecting each file.
  async function browseFolder(): Promise<void> {
    const rt = window.runtime;
    if (!rt?.Dialog?.Open) return;
    try {
      const selected = await rt.Dialog.Open({
        Title: tr('proto.browseDirTitle'),
        CanChooseDirectories: true,
        CanChooseFiles: false,
      });
      appendPaths(Array.isArray(selected) ? selected : selected ? [selected] : []);
    } catch {
      /* dialog cancelled */
    }
  }

  async function onImport(): Promise<void> {
    const paths = pathsText
      .split('\n')
      .map((s) => s.trim())
      .filter(Boolean);
    if (paths.length === 0) return;
    importing = true;
    const ok = await importProtos(paths);
    importing = false;
    if (ok) dispatch('close');
  }

  function onKeydown(e: KeyboardEvent): void {
    if (e.key === 'Escape') dispatch('close');
  }
</script>

<svelte:window on:keydown={onKeydown} />

<div class="overlay" role="presentation" on:click|self={() => dispatch('close')}>
  <div class="dialog" role="dialog" aria-modal="true">
    <div class="dialog-title">{$t('proto.title')}</div>
    <div class="dialog-body">
      <p class="hint">{$t('proto.hint')}</p>

      <div class="row">
        <span>{$t('proto.paths')} <em>{$t('proto.pathsHint')}</em></span>
        {#if hasNativeDialog}
          <span class="browse-group">
            <button class="btn btn-tiny" on:click={() => void browseFiles()}>{$t('proto.browse')}</button>
            <button class="btn btn-tiny" on:click={() => void browseFolder()}>{$t('proto.browseDir')}</button>
          </span>
        {/if}
      </div>
      <textarea
        class="mono paths"
        rows="4"
        bind:value={pathsText}
        spellcheck="false"
        placeholder={$t('proto.pathsPlaceholder')}
      />

      {#if $protoImportError}
        <div class="import-error">
          <div class="import-error-head">
            <strong>{$t('proto.errorTitle')}</strong>
            <span class="spacer" />
            <button class="btn btn-tiny" on:click={clearProtoImportError}>✕</button>
          </div>
          <div class="import-error-msg">{$protoImportError.message}</div>
          {#if $protoImportError.detail}
            <pre class="import-error-detail">{$protoImportError.detail}</pre>
          {/if}
        </div>
      {/if}

      <div class="imported">
        <div class="row">
          <span>{$t('proto.imported', { n: $importedProtos.length })}</span>
          {#if $importedProtos.length > 0}
            <button class="btn btn-tiny danger" on:click={() => void clearProtos()}>{$t('proto.clearAll')}</button>
          {/if}
        </div>
        {#if $importedProtos.length === 0}
          <div class="none">{$t('proto.none')}</div>
        {:else}
          <ul>
            {#each $importedProtos as f (f.name)}
              <li class="mono">
                {f.name}
                <span class="dim">package {f.package || '(none)'} · {f.messageCount} messages · {f.enumCount} enums</span>
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    </div>
    <div class="dialog-footer">
      <span class="spacer" />
      <button class="btn" on:click={() => dispatch('close')}>{$t('proto.close')}</button>
      <button class="btn primary" on:click={() => void onImport()} disabled={importing || pathsText.trim() === ''}>
        {importing ? $t('proto.importing') : $t('proto.import')}
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
    padding-top: 8vh;
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
  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    font-size: 11.5px;
    color: var(--dim);
    margin-bottom: 4px;
  }
  .row em {
    font-style: normal;
    opacity: 0.8;
  }
  .paths {
    width: 100%;
    font-size: 11.5px;
  }
  .browse-group {
    display: flex;
    gap: 4px;
  }
  .import-error {
    margin-top: 10px;
    border: 1px solid var(--danger);
    background: var(--danger-bg);
    border-radius: 4px;
    padding: 8px 10px;
  }
  .import-error-head {
    display: flex;
    align-items: center;
    font-size: 11.5px;
    color: var(--danger);
  }
  .import-error-msg {
    margin-top: 4px;
    font-size: 11.5px;
    line-height: 1.5;
    user-select: text;
  }
  .import-error-detail {
    margin: 6px 0 0;
    max-height: 180px;
    overflow: auto;
    font-family: var(--mono);
    font-size: 10.5px;
    line-height: 1.45;
    color: var(--dim);
    white-space: pre-wrap;
    word-break: break-all;
    user-select: text;
  }
  .import-error .spacer {
    flex: 1;
  }
  .imported {
    margin-top: 12px;
    border-top: 1px dashed var(--border);
    padding-top: 10px;
  }
  .imported ul {
    margin: 6px 0 0;
    padding: 0 0 0 4px;
    list-style: none;
    max-height: 180px;
    overflow-y: auto;
  }
  .imported li {
    font-size: 11.5px;
    padding: 2px 0;
    user-select: text;
  }
  .dim {
    color: var(--dim);
    font-size: 10.5px;
    margin-left: 6px;
  }
  .none {
    font-size: 11.5px;
    color: var(--dim);
    padding: 4px 0;
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
