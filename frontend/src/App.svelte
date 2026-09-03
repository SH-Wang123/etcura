<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import Toolbar from './components/Toolbar.svelte';
  import Sidebar from './components/Sidebar.svelte';
  import KeyTree from './components/KeyTree.svelte';
  import Inspector from './components/Inspector.svelte';
  import ConnectionDialog from './components/ConnectionDialog.svelte';
  import NewKeyDialog from './components/NewKeyDialog.svelte';
  import ConfirmDialog from './components/ConfirmDialog.svelte';
  import ProtoDialog from './components/ProtoDialog.svelte';
  import Toast from './components/Toast.svelte';
  import { loadConnections, refreshStatus, status } from './stores/connections';
  import { importedProtos, loadProtos } from './stores/protos';
  import {
    selectedPrefix,
    refreshCurrent,
    requestSave,
    updateDetail,
    bumpTree,
  } from './stores/keys';
  import { api } from './lib/api';
  import { t } from './lib/i18n';
  import { newConnectionForm } from './lib/types';
  import type { ConnectionConfig, KvDetail } from './lib/types';

  let searchEl: HTMLInputElement | null = null;

  // Connection dialog state
  let connectionDialogOpen = false;
  let dialogForm: ConnectionConfig = newConnectionForm();
  let editingHasPassword = false;

  // New key dialog state
  let newKeyOpen = false;

  // Proto import dialog state
  let protoDialogOpen = false;
  $: protoCount = $importedProtos.reduce((a, f) => a + f.messageCount, 0);

  onMount(() => {
    void loadConnections();
    void refreshStatus();
    void loadProtos();
  });

  function openAddConnection(): void {
    dialogForm = newConnectionForm();
    editingHasPassword = false;
    connectionDialogOpen = true;
  }

  async function openEditConnection(id: string): Promise<void> {
    dialogForm = await api.getConnection(id);
    editingHasPassword = true;
    connectionDialogOpen = true;
  }

  function onConnectionSaved(): void {
    connectionDialogOpen = false;
    void loadConnections();
  }

  async function onKeyCreated(e: CustomEvent<KvDetail>): Promise<void> {
    newKeyOpen = false;
    // Refresh the expanded branches in place so the tree keeps its shape; the
    // new key appears if its parent directory is expanded.
    bumpTree();
    updateDetail(e.detail);
  }

  function openNewKey(): void {
    if ($status.state !== 'connected') return;
    newKeyOpen = true;
  }

  function onRefreshAll(): void {
    if ($status.state === 'connected') {
      void refreshCurrent();
    }
    void refreshStatus();
  }

  function onKeydown(e: KeyboardEvent): void {
    const mod = e.metaKey || e.ctrlKey;
    if (!mod) return;
    const k = e.key.toLowerCase();
    if (k === 'r') {
      e.preventDefault();
      onRefreshAll();
    } else if (k === 'n') {
      e.preventDefault();
      openNewKey();
    } else if (k === 's') {
      e.preventDefault();
      requestSave();
    } else if (k === 'f') {
      e.preventDefault();
      searchEl?.focus();
      searchEl?.select();
    }
  }
</script>

<svelte:window on:keydown={onKeydown} />

<div class="app">
  <Toolbar bind:searchEl protoCount={protoCount} on:newkey={openNewKey} on:protos={() => (protoDialogOpen = true)} />
  <Sidebar on:add={openAddConnection} on:edit={(e) => void openEditConnection(e.detail)} />
  <KeyTree />
  <Inspector />

  <footer class="statusbar">
    <span class="dot {$status.state}" />
    <span class="state">{$status.state === 'connected' ? $t('status.connected') : $status.state === 'disconnected' ? $t('status.disconnected') : $status.state === 'connecting' ? $t('status.connecting') : $t('status.error')}</span>
    {#if $status.endpoints}
      <span class="sep">·</span>
      <span class="endpoints">{$status.endpoints}</span>
    {/if}
    {#if $status.state === 'connected'}
      <span class="sep">·</span>
      <span>rev {$status.header}</span>
      {#if $status.etcdVersion}
        <span class="sep">·</span>
        <span>etcd {$status.etcdVersion}</span>
      {/if}
    {/if}
    {#if $status.message}
      <span class="sep">·</span>
      <span class="err" title={$status.message}>{$status.message}</span>
    {/if}
    <span class="spacer" />
    <span class="hint">{$t('status.hint')}</span>
  </footer>
</div>

{#if connectionDialogOpen}
  <ConnectionDialog
    config={dialogForm}
    hasPassword={editingHasPassword}
    on:saved={onConnectionSaved}
    on:close={() => (connectionDialogOpen = false)}
  />
{/if}

{#if newKeyOpen}
  <NewKeyDialog defaultPrefix={$selectedPrefix} on:created={onKeyCreated} on:close={() => (newKeyOpen = false)} />
{/if}

{#if protoDialogOpen}
  <ProtoDialog on:close={() => (protoDialogOpen = false)} />
{/if}

<ConfirmDialog />
<Toast />

<style>
  .app {
    display: grid;
    height: 100vh;
    grid-template-rows: 42px 1fr 26px;
    grid-template-columns: 210px minmax(320px, 1fr) minmax(360px, 460px);
    grid-template-areas:
      'toolbar toolbar toolbar'
      'sidebar tree inspector'
      'status status status';
  }
  .statusbar {
    grid-area: status;
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 0 10px;
    font-size: 11px;
    font-family: var(--mono);
    color: var(--dim);
    border-top: 1px solid var(--border);
    background: var(--panel-alt);
    white-space: nowrap;
    overflow: hidden;
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--dim);
    flex: none;
  }
  .dot.connected {
    background: var(--ok);
  }
  .dot.connecting {
    background: var(--warning);
  }
  .dot.error {
    background: var(--danger);
  }
  .state {
    text-transform: capitalize;
  }
  .sep {
    opacity: 0.5;
  }
  .endpoints {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .err {
    color: var(--danger);
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .spacer {
    flex: 1;
  }
  .hint {
    opacity: 0.7;
  }
</style>
