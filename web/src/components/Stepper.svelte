<script lang="ts">
  import Check from '@lucide/svelte/icons/check';

  interface Props {
    steps: string[];
    current: number;
    /** Highest step the user may jump back to. */
    reachable: number;
    locked?: boolean;
    onstep?: (i: number) => void;
  }
  let { steps, current, reachable, locked = false, onstep }: Props = $props();
</script>

<nav aria-label="Steps">
  <ol>
    {#each steps as label, i (label)}
      {@const done = i < current}
      {@const active = i === current}
      {@const canGo = !locked && i !== current && i <= reachable}
      <li class:done class:active>
        <button
          type="button"
          disabled={!canGo}
          aria-current={active ? 'step' : undefined}
          onclick={() => onstep?.(i)}
        >
          <span class="dot">
            {#if done}<Check size={14} strokeWidth={3} aria-hidden={true} />{:else}{i + 1}{/if}
          </span>
          <span class="label">{label}</span>
          {#if done}<span class="sr-only">(done)</span>{/if}
        </button>
      </li>
    {/each}
  </ol>
</nav>

<style>
  ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    align-items: center;
    gap: 0.25rem;
  }
  li {
    display: flex;
    align-items: center;
    flex: 1;
    min-width: 0;
  }
  li:not(:last-child)::after {
    content: '';
    flex: 1;
    height: 2px;
    margin: 0 0.5rem;
    background: var(--border);
    border-radius: 2px;
    min-width: 12px;
  }
  li.done:not(:last-child)::after {
    background: var(--gold);
  }
  li:last-child {
    flex: none;
  }
  button {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    background: none;
    border: none;
    padding: 0.3rem 0.4rem 0.3rem 0.3rem;
    border-radius: 999px;
    color: var(--text-secondary);
    font-size: 0.9375rem;
    font-weight: 560;
    cursor: pointer;
  }
  button:disabled {
    cursor: default;
  }
  button:not(:disabled):hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }
  .dot {
    width: 28px;
    height: 28px;
    border-radius: 50%;
    display: grid;
    place-items: center;
    font-size: 0.8125rem;
    font-weight: 650;
    border: 1.5px solid var(--border-strong);
    background: var(--bg-raised);
    color: var(--text-secondary);
    flex: none;
    font-variant-numeric: tabular-nums;
  }
  .active .dot {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--on-accent);
  }
  .active button {
    color: var(--text-primary);
    font-weight: 650;
  }
  .done .dot {
    background: var(--success-soft);
    border-color: var(--success);
    color: var(--success-text);
  }
  .done button {
    color: var(--text-primary);
  }
  .label {
    white-space: nowrap;
  }
  @media (max-width: 760px) {
    li:not(.active) .label {
      display: none;
    }
  }
</style>
