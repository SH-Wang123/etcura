<script lang="ts">
  import {
    selectedKey,
    detail,
    detailLoading,
    saveSignal,
    reloadDetail,
    updateDetail,
    clearSelection,
    refreshCurrent,
  } from '../stores/keys';
  import { status } from '../stores/connections';
  import { importedProtos } from '../stores/protos';
  import {
    scriptInfo,
    scriptOutput,
    scriptRunning,
    scriptError,
    runScriptOver,
    resetScriptOutput,
  } from '../stores/script';
  import { api, localizeError } from '../lib/api';
  import { askConfirm } from '../lib/confirm';
  import { toast } from '../lib/toast';
  import { t, tr } from '../lib/i18n';
  import type { ProtoMatchResult } from '../lib/types';
  import {
    b64ToBytes,
    bytesToB64,
    toHex,
    fromHex,
    utf8Decode,
    utf8Encode,
    bytesEqual,
    parseJSON,
    prettyJSON,
    formatBytes,
  } from '../lib/base64';

  type ViewMode = 'text' | 'hex' | 'base64' | 'proto' | 'script';

  let mode: ViewMode = 'text';
  let text = '';
  let originalBytes: Uint8Array = new Uint8Array(0);
  let badges: string[] = [];

  // Protobuf auto-match state (proto view is display-only: it never feeds
  // back into the editor buffer, so saving stays byte-safe).
  let protoResult: ProtoMatchResult | null = null;
  let protoSelected = 0;
  let protoGen = 0;

  const LARGE_VALUE = 1024 * 1024;
  const PRETTY_JSON_MAX = 2 * 1024 * 1024;
  const PROTO_MATCH_MAX = 4 * 1024 * 1024;

  // Re-initialize the editor whenever a different key is loaded. This block
  // must NOT read `mode` anywhere (rendering uses a local copy) — reading it
  // would register a dependency, so every view switch would re-run this block
  // and instantly reset the mode (that was a real bug once).
  $: if ($detail) {
    // Binary values default to the post-processing script when one is
    // configured (its output is what the user asked to see); plain text keeps
    // the text view but can still switch to Script.
    const scriptActive = !!$scriptInfo;
    const initialMode: ViewMode = scriptActive && !$detail.isUTF8 ? 'script' : $detail.isUTF8 ? 'text' : 'hex';
    originalBytes = b64ToBytes($detail.valueBase64);
    mode = initialMode;
    text = render(originalBytes, initialMode === 'script' ? 'text' : initialMode);
    protoResult = null;
    protoSelected = 0;
    resetScriptOutput();
    void autoMatch($detail.valueBase64, $detail.sizeBytes, protoCount);
    if (scriptActive) void runScriptOver($detail.valueBase64);
  } else {
    originalBytes = new Uint8Array(0);
    text = '';
    protoResult = null;
    resetScriptOutput();
  }

  $: protoCount = $importedProtos.reduce((a, f) => a + f.messageCount, 0);

  function render(bytes: Uint8Array, m: ViewMode): string {
    if (m === 'hex') return toHex(bytes);
    if (m === 'base64') return bytesToB64(bytes);
    return utf8Decode(bytes);
  }

  async function autoMatch(valueBase64: string, size: number, count: number): Promise<void> {
    if (count === 0 || size === 0 || size > PROTO_MATCH_MAX) return;
    const gen = ++protoGen;
    try {
      const res = await api.matchProto(valueBase64);
      if (gen !== protoGen) return;
      protoResult = res.candidates.length > 0 ? res : null;
      protoSelected = 0;
    } catch {
      /* matching is best effort */
    }
  }

  $: protoCandidate = protoResult && protoSelected < protoResult.candidates.length
    ? protoResult.candidates[protoSelected]
    : null;
  $: protoText = protoCandidate?.json
    ? (prettyJSON(protoCandidate.json) ?? protoCandidate.json)
    : $t('inspector.protoTooLarge');

  // One-line description of the active binding, shown above the Script view.
  $: scriptBindingNote = (() => {
    const info = $scriptInfo;
    if (!info) return '';
    const parts: string[] = [$t('script.modeTip')];
    parts.push(info.inputVar ? $t('script.inputNote', { v: info.inputVar }) : $t('script.inputNoteEmpty'));
    parts.push(info.outputVar ? $t('script.outputNote', { v: info.outputVar }) : $t('script.outputNoteEmpty'));
    return parts.join(' ');
  })();

  // What the Script view shows: stdout, or the failure details.
  $: scriptText = (() => {
    if ($scriptError) return `${$t('script.failed')}: ${$scriptError}`;
    const out = $scriptOutput;
    if (!out) return '';
    if (out.exitCode !== 0) {
      const body = out.stderr.trim() || out.output.trim();
      return `${$t('script.failed')} (${$t('script.exitCode', { code: out.exitCode, ms: out.durationMs })})\n\n${body}`;
    }
    return out.output;
  })();

  function switchMode(m: ViewMode): void {
    if (m === mode) return;
    if (mode !== 'proto' && mode !== 'script') {
      // Leaving an editable mode: carry the buffer over to the new encoding.
      const current = encodeBytes(mode, text);
      if (!current) return;
      text = render(current, m);
    }
    // Leaving 'proto': the edit buffer was never touched, just switch back.
    mode = m;
  }

  // encodeBytes converts editor content back to raw bytes for an editable
  // view mode. Returns null when the text is malformed (bad hex/base64) or
  // the mode is display-only ('proto'). It takes its inputs as parameters so
  // the reactive statements below re-run when the editor changes — a
  // statement calling a closure would never see `text`/`mode` updates (no
  // syntactic dependency).
  function encodeBytes(m: ViewMode, s: string): Uint8Array | null {
    if (m === 'proto' || m === 'script') {
      return null;
    }
    if (m === 'hex') {
      return fromHex(s);
    }
    if (m === 'base64') {
      try {
        return b64ToBytes(s.replace(/\s+/g, ''));
      } catch {
        return null;
      }
    }
    return utf8Encode(s);
  }

  $: currentBytes = encodeBytes(mode, text);
  $: invalidInput = currentBytes === null;
  $: dirty = currentBytes !== null && !bytesEqual(currentBytes, originalBytes);
  $: isJSON = mode === 'text' && parseJSON(text) !== undefined;
  $: badges = $detail
    ? ([
        !$detail.isUTF8 ? $t('badge.binary') : null,
        $detail.sizeBytes > LARGE_VALUE ? $t('badge.large', { size: formatBytes($detail.sizeBytes) }) : null,
        isJSON ? $t('badge.json') : null,
        protoCandidate ? $t('badge.proto', { name: protoCandidate.messageName }) : null,
        mode === 'script' && $scriptOutput && $scriptOutput.exitCode === 0 && $scriptOutput.truncated
          ? $t('script.truncated')
          : null,
      ].filter((b): b is string => b !== null))
    : [];

  async function save(): Promise<void> {
    const d = $detail;
    if (!d || !dirty || invalidInput || $detailLoading) return;
    try {
      const saved = await api.saveKey(d.key, bytesToB64(currentBytes as Uint8Array), d.modRevision);
      updateDetail(saved);
      originalBytes = b64ToBytes(saved.valueBase64);
      toast(tr('inspector.savedToast'), 'success');
    } catch (e) {
      const err = localizeError(e);
      if (err.code === 'REVISION_CONFLICT') {
        const ok = await askConfirm({
          title: tr('inspector.conflictTitle'),
          lines: [err.message, '', tr('inspector.conflictQuestion')],
          confirmLabel: tr('inspector.reload'),
        });
        if (ok) await reloadDetail();
      } else {
        toast(err.message, 'error', err.detail);
      }
    }
  }

  async function remove(): Promise<void> {
    const d = $detail;
    if (!d) return;
    const ok = await askConfirm({
      title: tr('inspector.deleteTitle'),
      lines: [
        tr('inspector.deleteServer', { name: $status.name || $status.endpoints }),
        tr('inspector.deleteKeyLine', { key: d.key }),
        '',
        tr('inspector.cannotUndo'),
      ],
      confirmLabel: tr('btn.delete'),
      danger: true,
    });
    if (!ok) return;
    try {
      await api.deleteKey(d.key);
      clearSelection();
      toast(tr('inspector.deletedToast', { key: d.key }), 'success');
      await refreshCurrent();
    } catch (e) {
      const err = localizeError(e);
      toast(err.message, 'error', err.detail);
    }
  }

  function doPrettyJSON(): void {
    if (!isJSON) return;
    const pretty = prettyJSON(text);
    if (pretty !== null) text = pretty;
  }

  // ⌘S arrives through this store (see App.svelte).
  $: if ($saveSignal > 0) void save();
</script>

<div class="inspector">
  {#if !$selectedKey}
    <div class="empty">{$t('inspector.empty')}</div>
  {:else if $detailLoading && !$detail}
    <div class="empty"><span class="spinner" /> {$t('inspector.loading')}</div>
  {:else if $detail}
    <div class="insp-header">
      <div class="key-line" title={$detail.key}>{$detail.key}</div>
      <div class="badges">
        {#each badges as b (b)}
          <span class="badge" class:warn={b.startsWith('Large') || b === 'Binary value'}>{b}</span>
        {/each}
        <span class="badge size">{formatBytes($detail.sizeBytes)}</span>
      </div>
    </div>

    <div class="meta">
      <span title="Create Revision">create {Math.max($detail.createRevision, 0)}</span>
      <span title="Mod Revision">mod {$detail.modRevision}</span>
      <span title="Version">version {$detail.version}</span>
      <span title="Lease">lease {$detail.lease > 0 ? $detail.lease : '—'}</span>
    </div>

    <div class="value-toolbar">
      <div class="modes">
        <button class="btn btn-tiny" class:active={mode === 'text'} disabled={!$detail.isUTF8} on:click={() => switchMode('text')} title={$detail.isUTF8 ? $t('mode.tipText') : $t('mode.tipNotUTF8')}>{$t('mode.text')}</button>
        <button class="btn btn-tiny" class:active={mode === 'hex'} on:click={() => switchMode('hex')}>{$t('mode.hex')}</button>
        <button class="btn btn-tiny" class:active={mode === 'base64'} on:click={() => switchMode('base64')}>{$t('mode.base64')}</button>
        {#if protoResult}
          <button class="btn btn-tiny" class:active={mode === 'proto'} on:click={() => switchMode('proto')} title={$t('inspector.protoNote')}>
            {$t('mode.proto')}
          </button>
          <select
            class="proto-pick mono"
            bind:value={protoSelected}
            title={$t('inspector.candidateTip')}
          >
            {#each protoResult.candidates as c, idx (c.messageName)}
              <option value={idx}>{c.messageName} ({c.score})</option>
            {/each}
          </select>
        {/if}
        {#if $scriptInfo}
          <button
            class="btn btn-tiny"
            class:active={mode === 'script'}
            on:click={() => switchMode('script')}
            title={$t('script.modeTip')}
          >
            {$t('btn.script')}
          </button>
          {#if $scriptRunning}
            <span class="spinner" />
          {/if}
        {/if}
      </div>
      <span class="spacer" />
      {#if mode === 'text'}
        <button class="btn btn-tiny" disabled={!isJSON || $detail.sizeBytes > PRETTY_JSON_MAX} on:click={doPrettyJSON} title={$t('tip.prettyJSON')}>
          {$t('btn.prettyJSON')}
        </button>
      {/if}
      <button class="btn btn-tiny" on:click={() => void reloadDetail()} title={$t('tip.reload')}>{$t('btn.reload')}</button>
      <button class="btn btn-tiny danger" on:click={() => void remove()} title={$t('tip.delete')}>{$t('btn.delete')}</button>
      <button class="btn btn-tiny primary" disabled={!dirty || invalidInput} on:click={() => void save()} title={$t('tip.save')}>
        {dirty ? $t('btn.save') : $t('btn.saved')}
      </button>
    </div>

    {#if invalidInput && mode !== 'proto'}
      <div class="invalid">{mode === 'hex' ? $t('inspector.invalidHex') : $t('inspector.invalidBase64')}</div>
    {/if}
    {#if mode === 'proto'}
      <div class="invalid proto-note">{$t('inspector.protoNote')}</div>
    {/if}
    {#if mode === 'script'}
      <div class="invalid proto-note">{scriptBindingNote}</div>
    {/if}

    {#if mode === 'script'}
      <textarea
        class="editor mono"
        readonly
        value={$scriptRunning ? $t('script.running') : scriptText}
        wrap="soft"
        spellcheck="false"
      />
    {:else if mode === 'proto'}
      <textarea
        class="editor mono"
        readonly
        value={protoText}
        wrap="soft"
        spellcheck="false"
      />
    {:else}
      <textarea
        class="editor mono"
        bind:value={text}
        wrap={mode === 'text' ? 'soft' : 'off'}
        spellcheck="false"
        autocomplete="off"
      />
    {/if}
  {/if}
</div>

<style>
  .inspector {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0; /* grid item: allow shrinking below content height */
    min-width: 0;
    background: var(--panel);
  }
  .empty {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    color: var(--dim);
    font-size: 12px;
  }
  .spinner {
    width: 12px;
    height: 12px;
    border: 2px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
    display: inline-block;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .insp-header {
    padding: 8px 10px 4px;
    border-bottom: 1px solid var(--border);
    flex: none;
  }
  .key-line {
    font-family: var(--mono);
    font-size: 12.5px;
    font-weight: 600;
    word-break: break-all;
    user-select: text;
  }
  .badges {
    display: flex;
    gap: 6px;
    margin-top: 4px;
    flex-wrap: wrap;
  }
  .badge {
    font-size: 10px;
    padding: 1px 6px;
    border-radius: 3px;
    border: 1px solid var(--border);
    color: var(--dim);
  }
  .badge.warn {
    color: var(--warning-text);
    border-color: var(--warning);
    background: var(--warning-bg);
  }
  .badge.size {
    font-family: var(--mono);
  }
  .meta {
    display: flex;
    gap: 14px;
    padding: 5px 10px;
    font-family: var(--mono);
    font-size: 10.5px;
    color: var(--dim);
    border-bottom: 1px solid var(--border);
    background: var(--panel-alt);
    flex: none;
    flex-wrap: wrap;
  }
  .value-toolbar {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 5px 8px;
    border-bottom: 1px solid var(--border);
    flex: none;
  }
  .modes {
    display: flex;
    gap: 3px;
  }
  .spacer {
    flex: 1;
  }
  .invalid {
    padding: 4px 10px;
    font-size: 11px;
    color: var(--danger);
    background: var(--danger-bg);
    flex: none;
  }
  .proto-note {
    color: var(--warning-text);
    background: var(--warning-bg);
  }
  .proto-pick {
    max-width: 220px;
    font-size: 10.5px;
    padding: 1px 4px;
    height: 20px;
  }
  .editor {
    flex: 1;
    resize: none;
    border: none;
    outline: none;
    padding: 8px 10px;
    background: var(--panel);
    color: var(--text);
    font-size: 12px;
    line-height: 1.5;
    min-height: 0;
    user-select: text;
  }
</style>
