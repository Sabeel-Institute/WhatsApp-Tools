<script lang="ts">
  interface Props {
    value: number; // 0..1
    label: string;
    tone?: 'accent' | 'success' | 'warning' | 'muted';
    size?: 'sm' | 'md' | 'lg';
    indeterminate?: boolean;
  }
  let { value, label, tone = 'accent', size = 'md', indeterminate = false }: Props = $props();
  const pct = $derived(Math.round(Math.max(0, Math.min(1, value)) * 100));
</script>

<div
  class="track {size} {tone}"
  role="progressbar"
  aria-label={label}
  aria-valuemin={0}
  aria-valuemax={100}
  aria-valuenow={indeterminate ? undefined : pct}
>
  {#if indeterminate}
    <div class="bar indeterminate"></div>
  {:else}
    <div class="bar" style:width="{pct}%"></div>
  {/if}
</div>

<style>
  .track {
    width: 100%;
    background: var(--bg-inset);
    border-radius: 999px;
    overflow: hidden;
    position: relative;
  }
  .sm {
    height: 6px;
  }
  .md {
    height: 10px;
  }
  .lg {
    height: 14px;
  }
  .bar {
    height: 100%;
    border-radius: 999px;
    background: var(--accent);
    transition: width 0.5s ease;
  }
  .success .bar {
    background: var(--success);
  }
  .warning .bar {
    background: var(--warning);
  }
  .muted .bar {
    background: var(--border-strong);
  }
  .indeterminate {
    width: 35%;
    position: absolute;
    animation: slide 1.3s ease-in-out infinite;
  }
  @keyframes slide {
    from {
      left: -35%;
    }
    to {
      left: 100%;
    }
  }
</style>
