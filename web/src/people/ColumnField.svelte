<script lang="ts">
  import Sparkles from '@lucide/svelte/icons/sparkles';
  import Select from '../components/Select.svelte';
  import type { Column } from '../lib/types';

  interface Props {
    label: string;
    hint?: string;
    value: number;
    columns: Column[];
    suggested: boolean;
    onchange: () => void;
  }
  let { label, hint, value = $bindable(), columns, suggested, onchange }: Props = $props();

  const options = $derived(columns.map((c) => ({ value: c.index, label: c.label })));
  const samples = $derived(value >= 0 ? (columns[value]?.samples ?? []) : []);
</script>

<Select {label} {hint} bind:value {options} placeholder="Choose a column…" onchange={() => onchange()}>
  {#snippet tag()}
    {#if suggested && value >= 0}
      <span class="suggested"><Sparkles size={13} aria-hidden={true} /> Suggested</span>
    {/if}
  {/snippet}
  {#snippet below()}
    {#if value >= 0}
      <div class="samples" aria-label="Examples from this column">
        {#if samples.length}
          <span class="samples-label">Examples:</span>
          {#each samples as s, i (i)}<span class="sample">{s}</span>{/each}
        {:else}
          <span class="samples-label">This column is empty.</span>
        {/if}
      </div>
    {/if}
  {/snippet}
</Select>

<style>
  .suggested {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    font-size: 0.75rem;
    font-weight: 650;
    color: var(--warning-text);
    background: var(--warning-soft);
    border-radius: 999px;
    padding: 0.12rem 0.5rem;
  }
  .samples {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
    align-items: center;
    margin-top: 0.15rem;
  }
  .samples-label {
    font-size: 0.8125rem;
    color: var(--text-secondary);
    margin-right: 0.15rem;
  }
  .sample {
    font-size: 0.8125rem;
    background: var(--bg-inset);
    border-radius: 6px;
    padding: 0.12rem 0.5rem;
    max-width: 220px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-variant-numeric: tabular-nums;
  }
</style>
