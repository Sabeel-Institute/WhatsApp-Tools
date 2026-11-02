<script lang="ts">
  import type { Component } from 'svelte';

  interface Props {
    tone?: 'success' | 'warning' | 'danger' | 'info' | 'neutral' | 'accent';
    icon?: Component<{ size?: number; 'aria-hidden'?: boolean }>;
    text: string;
    size?: 'sm' | 'md';
  }
  let { tone = 'neutral', icon: Icon, text, size = 'sm' }: Props = $props();
</script>

<span class="pill {tone} {size}">
  {#if Icon}<Icon size={size === 'sm' ? 14 : 16} aria-hidden={true} />{/if}
  {text}
</span>

<style>
  .pill {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    border-radius: 999px;
    font-weight: 580;
    white-space: nowrap;
    line-height: 1.2;
  }
  .sm {
    font-size: 0.8125rem;
    padding: 0.22rem 0.6rem 0.22rem 0.5rem;
  }
  .md {
    font-size: 0.875rem;
    padding: 0.32rem 0.75rem 0.32rem 0.6rem;
  }
  .success {
    background: var(--success-soft);
    color: var(--success-text);
  }
  .warning {
    background: var(--warning-soft);
    color: var(--warning-text);
  }
  .danger {
    background: var(--danger-soft);
    color: var(--danger);
  }
  .info {
    background: var(--info-soft);
    color: var(--info);
  }
  .neutral {
    background: var(--neutral-soft);
    color: var(--text-secondary);
  }
  .accent {
    background: var(--bg-accent-soft);
    color: var(--accent);
  }
</style>
