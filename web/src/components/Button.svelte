<script lang="ts">
  import type { Component, Snippet } from 'svelte';
  import Spinner from './Spinner.svelte';

  interface Props {
    variant?: 'primary' | 'secondary' | 'ghost' | 'danger' | 'quiet';
    size?: 'sm' | 'md' | 'lg';
    type?: 'button' | 'submit';
    disabled?: boolean;
    loading?: boolean;
    icon?: Component<{ size?: number; 'aria-hidden'?: boolean }>;
    iconRight?: Component<{ size?: number; 'aria-hidden'?: boolean }>;
    full?: boolean;
    title?: string;
    ariaLabel?: string;
    onclick?: (e: MouseEvent) => void;
    children?: Snippet;
  }

  let {
    variant = 'secondary',
    size = 'md',
    type = 'button',
    disabled = false,
    loading = false,
    icon: Icon,
    iconRight: IconRight,
    full = false,
    title,
    ariaLabel,
    onclick,
    children,
  }: Props = $props();

  const iconSize = $derived(size === 'lg' ? 20 : size === 'sm' ? 16 : 18);
</script>

<button
  {type}
  class="btn {variant} {size}"
  class:full
  disabled={disabled || loading}
  aria-busy={loading || undefined}
  aria-label={ariaLabel}
  {title}
  {onclick}
>
  {#if loading}
    <Spinner size={iconSize} />
  {:else if Icon}
    <Icon size={iconSize} aria-hidden={true} />
  {/if}
  {#if children}<span>{@render children()}</span>{/if}
  {#if IconRight && !loading}
    <IconRight size={iconSize} aria-hidden={true} />
  {/if}
</button>

<style>
  .btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    border: 1px solid transparent;
    border-radius: 999px;
    font-weight: 600;
    line-height: 1.2;
    cursor: pointer;
    white-space: nowrap;
    transition:
      background-color 0.15s ease,
      border-color 0.15s ease,
      color 0.15s ease,
      box-shadow 0.15s ease,
      transform 0.05s ease;
  }
  .btn:active:not(:disabled) {
    transform: translateY(1px);
  }
  .btn:focus-visible {
    outline: none;
    box-shadow: var(--focus);
  }
  .btn:disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }
  .full {
    width: 100%;
  }
  .sm {
    padding: 0.4rem 0.85rem;
    font-size: 0.875rem;
    min-height: 34px;
  }
  .md {
    padding: 0.6rem 1.15rem;
    font-size: 0.9375rem;
    min-height: 42px;
  }
  .lg {
    padding: 0.8rem 1.5rem;
    font-size: 1rem;
    min-height: 50px;
  }
  .primary {
    background: var(--accent);
    color: var(--on-accent);
    box-shadow: 0 1px 2px rgba(82, 10, 50, 0.25);
  }
  .primary:hover:not(:disabled) {
    background: var(--accent-hover);
  }
  .primary:active:not(:disabled) {
    background: var(--accent-pressed);
  }
  .secondary {
    background: var(--bg-raised);
    color: var(--text-primary);
    border-color: var(--border-strong);
  }
  .secondary:hover:not(:disabled) {
    border-color: var(--text-secondary);
    background: #fffdfa;
  }
  .ghost {
    background: transparent;
    color: var(--text-accent);
  }
  .ghost:hover:not(:disabled) {
    background: var(--bg-accent-soft);
  }
  .quiet {
    background: transparent;
    color: var(--text-secondary);
  }
  .quiet:hover:not(:disabled) {
    background: var(--bg-hover);
    color: var(--text-primary);
  }
  .danger {
    background: var(--bg-raised);
    color: var(--danger);
    border-color: #e2b8b3;
  }
  .danger:hover:not(:disabled) {
    background: var(--danger-soft);
    border-color: var(--danger);
  }
</style>
