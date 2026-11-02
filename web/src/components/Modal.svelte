<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    open: boolean;
    title: string;
    onclose: () => void;
    children?: Snippet;
    actions?: Snippet;
    width?: number;
  }
  let { open, title, onclose, children, actions, width = 480 }: Props = $props();
  let dialog: HTMLDialogElement | undefined = $state();
  const titleId = `modal-${Math.random().toString(36).slice(2)}`;

  $effect(() => {
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) dialog.close();
  });
</script>

<dialog
  bind:this={dialog}
  aria-labelledby={titleId}
  style:max-width="min({width}px, calc(100vw - 32px))"
  oncancel={(e) => {
    e.preventDefault();
    onclose();
  }}
  onclick={(e) => {
    if (e.target === dialog) onclose();
  }}
>
  {#if open}
    <div class="content">
      <h2 id={titleId}>{title}</h2>
      {#if children}<div class="body">{@render children()}</div>{/if}
      {#if actions}<div class="actions">{@render actions()}</div>{/if}
    </div>
  {/if}
</dialog>

<style>
  dialog {
    border: none;
    padding: 0;
    border-radius: var(--radius-lg);
    background: var(--bg-raised);
    color: var(--text-primary);
    box-shadow: var(--shadow-lg);
    width: 100%;
  }
  dialog::backdrop {
    background: var(--backdrop);
    backdrop-filter: blur(2px);
  }
  dialog[open] {
    animation: pop 0.16s ease-out;
  }
  @keyframes pop {
    from {
      opacity: 0;
      transform: translateY(6px) scale(0.98);
    }
  }
  .content {
    padding: 1.6rem 1.6rem 1.3rem;
  }
  h2 {
    font-size: 1.25rem;
  }
  .body {
    margin-top: 0.6rem;
    color: var(--text-secondary);
  }
  .body :global(p + p) {
    margin-top: 0.6rem;
  }
  .actions {
    margin-top: 1.5rem;
    display: flex;
    justify-content: flex-end;
    gap: 0.6rem;
    flex-wrap: wrap;
  }
</style>
