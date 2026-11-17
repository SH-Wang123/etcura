<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import Toolbar from './Toolbar.svelte';
  import KeyTree from './KeyTree.svelte';
  import Inspector from './Inspector.svelte';
  import NewKeyDialog from './NewKeyDialog.svelte';
  import { provideBrowser } from '../stores/keys';
  import { refreshStatus } from '../stores/connections';
  import { importedProtos } from '../stores/protos';
  import { scriptInfo } from '../stores/script';
  import type { KvDetail, StatusInfo } from '../lib/types';
  import { t } from '../lib/i18n';

  export let connection: StatusInfo;
  export let active: boolean;
  const dispatch = createEventDispatcher<{ protos: void; script: void }>();
  const { status, selectedPrefix, refreshAll, requestSave, updateDetail, bumpTree } = provideBrowser(connection);
  $: status.set(connection);
  $: protoCount = $importedProtos.reduce((total, file) => total + file.messageCount, 0);

  let searchEl: HTMLInputElement | null = null;
  let newKeyOpen = false;

  function onKeyCreated(event: CustomEvent<KvDetail>): void {
    newKeyOpen = false;
    bumpTree();
    updateDetail(event.detail);
  }

  function onKeydown(event: KeyboardEvent): void {
    if (!active || event.defaultPrevented || !(event.metaKey || event.ctrlKey)) return;
    switch (event.key.toLowerCase()) {
      case 'r': void refreshAll(); void refreshStatus(); break;
      case 'n': if (!connection.readOnly) newKeyOpen = true; break;
      case 's': requestSave(); break;
      case 'f': searchEl?.focus(); searchEl?.select(); break;
      default: return;
    }
    event.preventDefault();
  }
</script>

<svelte:window on:keydown={onKeydown} />
<section class="pane" hidden={!active}>
  <Toolbar bind:searchEl {protoCount} scriptName={$scriptInfo?.language ?? ''}
    on:newkey={() => (newKeyOpen = true)} on:protos on:script />
  <KeyTree />
  <Inspector />
  <footer>
    <strong>{connection.name}</strong>
    {#if connection.environment}<span>{connection.environment}</span>{/if}
    {#if connection.readOnly}<span>{$t('conn.readOnly')}</span>{/if}
    <span class="endpoint" title={connection.endpoints}>{connection.endpoints}</span>
    <span>rev {connection.header}</span>
    <span>etcd {connection.etcdVersion}</span>
  </footer>
</section>
{#if active && newKeyOpen}
  <NewKeyDialog session={connection.session} defaultPrefix={$selectedPrefix}
    on:created={onKeyCreated} on:close={() => (newKeyOpen = false)} />
{/if}

<style>
  .pane {
    display: grid;
    min-height: 0;
    height: 100%;
    grid-template: "toolbar toolbar" 42px "tree inspector" minmax(0, 1fr) "status status" 28px / minmax(230px, 1fr) minmax(360px, 1.2fr);
  }
  .pane[hidden] { display: none; }
  footer {
    grid-area: status;
    display: flex;
    gap: 12px;
    align-items: center;
    padding: 0 12px;
    border-top: 1px solid var(--border);
    color: var(--dim);
    font-size: 11px;
    white-space: nowrap;
  }
  footer strong { color: var(--text); }
  .endpoint { min-width: 0; overflow: hidden; text-overflow: ellipsis; margin-right: auto; }
</style>
