<script lang="ts">
  import { toasts, dismissToast } from '../lib/toast';
</script>

<div class="toasts">
  {#each $toasts as t (t.id)}
    <div class="toast {t.kind}" role="status" on:click={() => dismissToast(t.id)}>
      <div class="message">{t.message}</div>
      {#if t.detail}
        <div class="detail">{t.detail}</div>
      {/if}
    </div>
  {/each}
</div>

<style>
  .toasts {
    position: fixed;
    top: 40px;
    right: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    z-index: 80;
    max-width: 420px;
  }
  .toast {
    background: var(--panel);
    border: 1px solid var(--border);
    border-left: 3px solid var(--accent);
    border-radius: 6px;
    padding: 8px 12px;
    box-shadow: 0 6px 24px rgba(0, 0, 0, 0.25);
    cursor: default;
  }
  .toast.error {
    border-left-color: var(--danger);
  }
  .toast.success {
    border-left-color: var(--ok);
  }
  .message {
    font-size: 12px;
    font-weight: 500;
  }
  .detail {
    margin-top: 3px;
    font-size: 11px;
    color: var(--dim);
    word-break: break-all;
    max-height: 80px;
    overflow-y: auto;
  }
</style>
