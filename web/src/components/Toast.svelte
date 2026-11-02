<script lang="ts">
  import CircleCheck from '@lucide/svelte/icons/circle-check';
  import { toast } from './toaster.svelte';
</script>

<div class="host" role="status" aria-live="polite">
  {#if toast.message}
    <div class="toast"><CircleCheck size={18} aria-hidden={true} /> {toast.message}</div>
  {/if}
</div>

<style>
  :global(html[data-capture]:not([data-capture='toast'])) .host {
    display: none;
  }
  .host {
    position: fixed;
    bottom: 1.5rem;
    left: 50%;
    transform: translateX(-50%);
    z-index: 100;
    pointer-events: none;
  }
  .toast {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    background: var(--text-primary);
    color: var(--text-inverse);
    padding: 0.7rem 1.1rem;
    border-radius: 999px;
    box-shadow: var(--shadow-lg);
    font-weight: 560;
    font-size: 0.9375rem;
    animation: rise 0.18s ease-out;
  }
  @keyframes rise {
    from {
      opacity: 0;
      transform: translateY(8px);
    }
  }
</style>
