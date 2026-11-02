<script lang="ts">
  import type { Snippet } from 'svelte';
  import Info from '@lucide/svelte/icons/info';
  import CircleCheck from '@lucide/svelte/icons/circle-check';
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
  import CircleAlert from '@lucide/svelte/icons/circle-alert';
  import Lightbulb from '@lucide/svelte/icons/lightbulb';

  interface Props {
    tone?: 'info' | 'success' | 'warning' | 'danger' | 'tip';
    title?: string;
    children?: Snippet;
    actions?: Snippet;
    live?: boolean;
  }
  let { tone = 'info', title, children, actions, live = false }: Props = $props();
  const icons = { info: Info, success: CircleCheck, warning: TriangleAlert, danger: CircleAlert, tip: Lightbulb };
  const Icon = $derived(icons[tone]);
</script>

<div class="banner {tone}" role={live ? (tone === 'danger' ? 'alert' : 'status') : undefined}>
  <span class="icon"><Icon size={20} aria-hidden={true} /></span>
  <div class="body">
    {#if title}<p class="title">{title}</p>{/if}
    {#if children}<div class="text">{@render children()}</div>{/if}
    {#if actions}<div class="actions">{@render actions()}</div>{/if}
  </div>
</div>

<style>
  .banner {
    display: flex;
    gap: 0.75rem;
    padding: 0.9rem 1rem;
    border-radius: var(--radius);
    border: 1px solid;
    align-items: flex-start;
  }
  .icon {
    display: flex;
    margin-top: 1px;
    flex: none;
  }
  .body {
    min-width: 0;
    flex: 1;
  }
  .title {
    font-weight: 620;
  }
  .title + .text {
    margin-top: 0.15rem;
  }
  .text {
    color: var(--text-secondary);
    font-size: 0.9375rem;
  }
  .actions {
    margin-top: 0.75rem;
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }
  .info {
    background: var(--info-soft);
    border-color: var(--border-info);
  }
  .info .icon {
    color: var(--info);
  }
  .success {
    background: var(--success-soft);
    border-color: var(--border-success);
  }
  .success .icon {
    color: var(--success);
  }
  .warning {
    background: var(--warning-soft);
    border-color: var(--border-warning);
  }
  .warning .icon {
    color: var(--warning);
  }
  .danger {
    background: var(--danger-soft);
    border-color: var(--border-danger);
  }
  .danger .icon {
    color: var(--danger);
  }
  .tip {
    background: var(--bg-surface);
    border-color: var(--border);
  }
  .tip .icon {
    color: var(--warning);
  }
</style>
