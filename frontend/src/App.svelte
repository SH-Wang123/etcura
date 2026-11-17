<script lang="ts">
  import { onMount } from 'svelte';
  import Sidebar from './components/Sidebar.svelte';
  import ConnectionDialog from './components/ConnectionDialog.svelte';
  import ConfirmDialog from './components/ConfirmDialog.svelte';
  import ProtoDialog from './components/ProtoDialog.svelte';
  import ScriptDialog from './components/ScriptDialog.svelte';
  import Toast from './components/Toast.svelte';
  import SessionPane from './components/SessionPane.svelte';
  import ComparePane from './components/ComparePane.svelte';
  import WatchPane from './components/WatchPane.svelte';
  import { connections, loadConnections, refreshStatus, activeSession, sessions, disconnect } from './stores/connections';
  import { loadProtos } from './stores/protos';
  import { loadScript } from './stores/script';
  import { api, localizeError } from './lib/api';
  import { toast } from './lib/toast';
  import { t, locale, locales, setLocale } from './lib/i18n';
  import { sessionLabel } from './lib/workbench';
  import { newConnectionForm, type ConnectionConfig } from './lib/types';

  let panel: 'browse' | 'compare' | 'watch' = 'browse';
  let connectionDialogOpen = false;
  let dialogForm: ConnectionConfig = newConnectionForm();
  let editingHasPassword = false;
  let protoDialogOpen = false;
  let scriptDialogOpen = false;
  $: if ($activeSession) selectSession($activeSession);

  onMount(() => {
    void loadConnections();
    void refreshStatus();
    void loadProtos();
    void loadScript();
  });

  function selectSession(id: string): void {
    if (id) panel = 'browse';
  }

  function openAddConnection(): void {
    dialogForm = newConnectionForm();
    editingHasPassword = false;
    connectionDialogOpen = true;
  }

  async function openEditConnection(id: string): Promise<void> {
    try {
      dialogForm = await api.getConnection(id);
      editingHasPassword = $connections.find((conn) => conn.id === id)?.hasPassword ?? false;
      connectionDialogOpen = true;
    } catch (error) {
      toast(localizeError(error).message, 'error');
    }
  }

  function onConnectionSaved(): void {
    connectionDialogOpen = false;
    void loadConnections();
    void refreshStatus();
  }
</script>

<div class="workbench">
  <Sidebar on:open={() => (panel = 'browse')} on:add={openAddConnection} on:edit={(event) => void openEditConnection(event.detail)} />
  <main>
    <header>
      <div class="tabs" aria-label={$t('work.openSessions')}>
        {#each $sessions as session (session.session)}
          <div class="tab" class:chosen={panel === 'browse' && $activeSession === session.session}>
            <button class="tab-name" title={session.endpoints} aria-pressed={panel === 'browse' && $activeSession === session.session}
              on:click={() => { activeSession.set(session.session); panel = 'browse'; }}>
              {sessionLabel(session)}{session.readOnly ? ` · ${$t('work.readOnlyShort')}` : ''}
            </button>
            <button class="close" aria-label={`${$t('sidebar.disconnect')} ${session.name}`} on:click={() => void disconnect(session.session)}>×</button>
          </div>
        {/each}
      </div>
      <nav>
        <button class="btn" class:active={panel === 'compare'} on:click={() => (panel = 'compare')}>{$t('work.compare')}</button>
        <button class="btn" class:active={panel === 'watch'} on:click={() => (panel = 'watch')}>{$t('work.watch')}</button>
        {#if !$sessions.length || panel !== 'browse'}
          <select value={$locale} aria-label={$t('locale.label')} on:change={(event) => setLocale(event.currentTarget.value as typeof $locale)}>
            {#each locales as item (item.id)}<option value={item.id}>{item.label}</option>{/each}
          </select>
        {/if}
      </nav>
    </header>

    <div class="content">
      {#each $sessions as session (session.session)}
        <SessionPane connection={session} active={panel === 'browse' && session.session === $activeSession}
          on:protos={() => (protoDialogOpen = true)} on:script={() => (scriptDialogOpen = true)} />
      {/each}
      {#if panel === 'browse' && !$sessions.length}
        <div class="empty"><h1>Etcura</h1><p>{$t('tree.connectFirst')}</p></div>
      {/if}
      <div class="tool" hidden={panel !== 'compare'}><ComparePane /></div>
      <div class="tool" hidden={panel !== 'watch'}><WatchPane /></div>
    </div>
  </main>
</div>

{#if connectionDialogOpen}
  <ConnectionDialog config={dialogForm} hasPassword={editingHasPassword} on:saved={onConnectionSaved} on:close={() => (connectionDialogOpen = false)} />
{/if}
{#if protoDialogOpen}<ProtoDialog on:close={() => (protoDialogOpen = false)} />{/if}
{#if scriptDialogOpen}<ScriptDialog on:close={() => (scriptDialogOpen = false)} />{/if}
<ConfirmDialog />
<Toast />

<style>
  .workbench { display: grid; grid-template: "sidebar workspace" minmax(0, 1fr) / 196px minmax(0, 1fr); height: 100vh; }
  main { grid-area: workspace; min-width: 0; min-height: 0; display: grid; grid-template-rows: 46px minmax(0,1fr); }
  header { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 0 10px; border-bottom: 1px solid var(--border); background: var(--panel-alt); }
  .tabs, nav { display: flex; align-items: center; gap: 6px; }
  nav { flex-shrink: 0; }
  .tabs { overflow-x: auto; min-width: 0; padding: 3px 0; }
  .tab { display: flex; flex-shrink: 0; align-items: center; border: 1px solid var(--border); border-radius: 6px; overflow: hidden; }
  .tab.chosen { background: var(--panel); border-color: var(--accent); }
  .tab button { background: none; border: 0; color: var(--text); padding: 6px 8px; cursor: pointer; }
  .tab-name { max-width: 160px; white-space: nowrap; text-overflow: ellipsis; overflow: hidden; }
  .tab .close { color: var(--dim); }
  .tab .close:hover { color: var(--danger); }
  .content, .tool { min-height: 0; height: 100%; }
  .tool[hidden] { display: none; }
  .empty { display: grid; place-content: center; text-align: center; height: 100%; color: var(--dim); }
  .empty h1 { margin: 0; color: var(--text); font-size: 26px; font-weight: 650; }
</style>
