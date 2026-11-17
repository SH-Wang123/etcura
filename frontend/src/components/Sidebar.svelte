<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import {
    connections,
    activeId,
    connectingId,
    connect,
    disconnect,
    loadConnections,
    refreshStatus,
    status,
  } from '../stores/connections';
  import { askConfirm } from '../lib/confirm';
  import { api, localizeError } from '../lib/api';
  import { toast } from '../lib/toast';
  import { t, tr } from '../lib/i18n';
  import type { ConnectionSummary } from '../lib/types';

  const dispatch = createEventDispatcher<{ edit: string; add: void }>();

  async function onDelete(c: ConnectionSummary): Promise<void> {
    const ok = await askConfirm({
      title: tr('sidebar.deleteTitle'),
      lines: [
        tr('sidebar.deleteName', { name: c.name }),
        tr('sidebar.deleteEndpoints', { endpoints: c.endpoints.join(', ') }),
        '',
        tr('sidebar.deleteKeychainNote'),
      ],
      confirmLabel: tr('sidebar.delete'),
      danger: true,
    });
    if (!ok) return;
    try {
      await api.deleteConnection(c.id);
      await Promise.all([loadConnections(), refreshStatus()]);
      toast(tr('sidebar.deletedToast', { name: c.name }), 'success');
    } catch (e) {
      const err = localizeError(e);
      toast(err.message, 'error', err.detail);
    }
  }

  function isActive(c: ConnectionSummary): boolean {
    return $activeId === c.id && $status.state !== 'disconnected';
  }
</script>

<div class="sidebar">
  <div class="side-header">
    <span class="title">{$t('sidebar.title')}</span>
    <button class="btn btn-small" title={$t('sidebar.addTitle')} on:click={() => dispatch('add')}>+</button>
  </div>

  {#if $connections.length === 0}
    <div class="empty">
      {$t('sidebar.empty1')}<br />
      <button class="btn" on:click={() => dispatch('add')}>{$t('sidebar.addConnection')}</button>
    </div>
  {:else}
    <div class="list">
      {#each $connections as c (c.id)}
        <div class="item" class:active={isActive(c)}>
          <div class="item-main" role="button" tabindex="0" on:click={() => void connect(c.id)} on:keydown={(e) => e.key === 'Enter' && void connect(c.id)}>
            <div class="name-row">
              <span class="name">{c.name}</span>
              {#if $connectingId === c.id}
                <span class="spinner" />
              {:else if isActive(c)}
                <span class="dot" title={$status.state} />
              {/if}
            </div>
            <div class="endpoints" title={c.endpoints.join(', ')}>
              {c.endpoints.join(', ')}
            </div>
          </div>
          <div class="item-actions">
            {#if $activeId === c.id}
              <button class="btn btn-tiny" on:click={() => void disconnect()}>{$t('sidebar.disconnect')}</button>
            {:else}
              <button class="btn btn-tiny" on:click={() => void connect(c.id)}>
                {$connectingId === c.id ? '…' : $t('sidebar.connect')}
              </button>
            {/if}
            <button class="btn btn-tiny" title={$t('sidebar.edit')} on:click={() => dispatch('edit', c.id)}>
              {$t('sidebar.edit')}
            </button>
            <button class="btn btn-tiny danger" title={$t('sidebar.delete')} on:click={() => void onDelete(c)}>{$t('sidebar.delete')}</button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .sidebar {
    grid-area: sidebar;
    display: flex;
    flex-direction: column;
    background: var(--panel-alt);
    border-right: 1px solid var(--border);
    min-height: 0;
  }
  .side-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 32px;
    padding: 0 8px;
    border-bottom: 1px solid var(--border);
    flex: none;
  }
  .title {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--dim);
  }
  .empty {
    padding: 24px 12px;
    color: var(--dim);
    font-size: 12px;
    text-align: center;
    line-height: 1.7;
  }
  .list {
    overflow-y: auto;
    flex: 1;
    padding: 4px;
  }
  .item {
    border: 1px solid transparent;
    border-radius: 5px;
    padding: 6px 8px;
    margin-bottom: 2px;
  }
  .item:hover {
    background: var(--hover);
  }
  .item.active {
    background: var(--selected);
    border-color: var(--accent);
  }
  .item-main {
    cursor: default;
    min-width: 0;
  }
  .name-row {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .name {
    font-size: 12.5px;
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--ok);
    flex: none;
  }
  .spinner {
    width: 10px;
    height: 10px;
    border: 1.5px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
    flex: none;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .endpoints {
    font-family: var(--mono);
    font-size: 10px;
    color: var(--dim);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    margin-top: 2px;
  }
  .item-actions {
    display: none;
    gap: 4px;
    margin-top: 6px;
  }
  .item:hover .item-actions {
    display: flex;
  }
</style>
