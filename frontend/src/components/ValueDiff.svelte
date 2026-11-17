<script lang="ts">
  import type { model } from '../../wailsjs/go/models';
  import { api, localizeError } from '../lib/api';
  import type { ProtoCandidate } from '../lib/types';
  import { importedProtos } from '../stores/protos';
  import { b64ToBytes, utf8Decode } from '../lib/base64';
  import { alignLines, jsonFields } from '../lib/valueDiff';
  import { t } from '../lib/i18n';

  export let left: model.RecordValue | null | undefined;
  export let right: model.RecordValue | null | undefined;
  export let leftLabel: string;
  export let rightLabel: string;
  let mode: 'raw' | 'fields' | 'proto' = 'raw';
  let protoName = '';
  let leftCandidates: ProtoCandidate[] = [];
  let rightCandidates: ProtoCandidate[] = [];
  let decoding = false;
  let issue = '';
  let generation = 0;

  function display(value: model.RecordValue | null | undefined): string {
    if (!value) return '';
    return value.utf8 ? utf8Decode(b64ToBytes(value.base64)) : value.base64;
  }

  async function changeMode(): Promise<void> {
    if (mode !== 'proto') return;
    const current = ++generation;
    decoding = true;
    issue = '';
    try {
      const [a, b] = await Promise.all([left, right].map((value) =>
        value && !value.truncated ? api.matchProto(value.base64).then((result) => result.candidates ?? []) : []));
      if (current !== generation) return;
      leftCandidates = a;
      rightCandidates = b;
      protoName = a[0]?.messageName ?? b[0]?.messageName ?? '';
    } catch (error) {
      if (current === generation) issue = localizeError(error).message;
    } finally {
      if (current === generation) decoding = false;
    }
  }

  $: leftText = display(left);
  $: rightText = display(right);
  $: leftFields = left?.utf8 && !left.truncated && left.bytes <= 256000 ? jsonFields(leftText) : null;
  $: rightFields = right?.utf8 && !right.truncated && right.bytes <= 256000 ? jsonFields(rightText) : null;
  $: names = [...new Set([...leftCandidates, ...rightCandidates].map((item) => item.messageName))];
  $: leftRendered = mode === 'proto' ? leftCandidates.find((item) => item.messageName === protoName)?.json ?? '' : leftText;
  $: rightRendered = mode === 'proto' ? rightCandidates.find((item) => item.messageName === protoName)?.json ?? '' : rightText;
  $: rows = alignLines(mode === 'fields' ? leftFields ?? (left ? leftText.split('\n') : []) : left ? leftRendered.split('\n') : [],
    mode === 'fields' ? rightFields ?? (right ? rightText.split('\n') : []) : right ? rightRendered.split('\n') : []);
</script>

<div class="modes">
  <select aria-label={$t('work.view')} bind:value={mode} on:change={() => void changeMode()}>
    <option value="raw">{$t('work.raw')}</option>
    {#if leftFields || rightFields}<option value="fields">{$t('work.jsonFields')}</option>{/if}
    {#if $importedProtos.length}<option value="proto">Protobuf</option>{/if}
  </select>
  {#if mode === 'proto' && names.length}
    <select aria-label={$t('inspector.candidateTip')} bind:value={protoName}>
      {#each names as name (name)}<option value={name}>{name}</option>{/each}
    </select>
  {/if}
  {#if mode === 'proto' && decoding}<span>{$t('work.loading')}</span>{/if}
  {#if mode === 'proto' && !decoding && !names.length}<span>{issue || $t('work.noProto')}</span>{/if}
</div>
<div class="heads">
  <div>{leftLabel}{left ? ` · ${left.bytes} B${left.utf8 ? '' : ' · Base64'}` : ` · ${$t('work.missing')}`}</div>
  <div>{rightLabel}{right ? ` · ${right.bytes} B${right.utf8 ? '' : ' · Base64'}` : ` · ${$t('work.missing')}`}</div>
</div>
{#if left?.truncated || right?.truncated}<p class="notice">{$t('work.truncated')}</p>{/if}
{#if mode !== 'proto' || names.length}
  <div class="lines">
    {#if rows}
      {#each rows as row}
        <div class="line" class:removed={row.changed && row.left !== undefined}>{row.left ?? ''}</div>
        <div class="line" class:added={row.changed && row.right !== undefined}>{row.right ?? ''}</div>
      {/each}
    {:else}
      <pre>{mode === 'fields' ? (leftFields?.join('\n') ?? leftText) : leftRendered}</pre>
      <pre>{mode === 'fields' ? (rightFields?.join('\n') ?? rightText) : rightRendered}</pre>
    {/if}
  </div>
{/if}

<style>
  .modes { display: flex; align-items: center; gap: 8px; padding: 0 0 10px; color: var(--dim); }
  .heads, .lines { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); }
  .heads { font-size: 12px; font-weight: 600; }
  .heads div { padding: 7px; border: 1px solid var(--border); background: var(--panel-alt); overflow-wrap: anywhere; }
  .lines { border: 1px solid var(--border); border-top: 0; }
  .line, pre { font-family: var(--mono); font-size: 11px; min-width: 0; margin: 0; min-height: 18px; padding: 2px 7px; white-space: pre-wrap; overflow-wrap: anywhere; }
  .line:nth-child(odd), pre:first-child { border-right: 1px solid var(--border); }
  .added { background: color-mix(in srgb, var(--ok) 13%, transparent); }
  .removed { background: color-mix(in srgb, var(--danger) 13%, transparent); }
  .notice { color: var(--warning); }
</style>
