<script lang="ts">
  import { confirmState, answerConfirm } from '../lib/confirm';
  import { t } from '../lib/i18n';
</script>

{#if $confirmState}
  <div class="overlay" role="presentation">
    <div class="dialog" class:danger={$confirmState.danger} role="alertdialog" aria-modal="true">
      <div class="dialog-title">{$confirmState.title}</div>
      <div class="dialog-body">{$confirmState.lines.join('\n')}</div>
      <div class="dialog-footer">
        <button class="btn" on:click={() => answerConfirm(false)}>
          {$confirmState.cancelLabel ?? $t('confirm.cancel')}
        </button>
        <button
          class="btn"
          class:danger={$confirmState.danger}
          class:primary={!$confirmState.danger}
          on:click={() => answerConfirm(true)}
        >
          {$confirmState.confirmLabel ?? $t('confirm.ok')}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.4);
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding-top: 14vh;
    z-index: 60;
  }
  .dialog {
    width: 460px;
    max-width: calc(100vw - 40px);
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    box-shadow: 0 18px 50px rgba(0, 0, 0, 0.4);
    overflow: hidden;
  }
  .dialog-title {
    padding: 12px 16px 0;
    font-size: 13px;
    font-weight: 600;
  }
  .dialog-body {
    padding: 8px 16px 4px;
    font-size: 12px;
    line-height: 1.6;
    white-space: pre-wrap;
    word-break: break-all;
    color: var(--text);
  }
  .dialog-footer {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding: 12px 16px 14px;
  }
</style>
