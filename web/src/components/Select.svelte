<script lang="ts" generics="T extends string | number">
  import type { Snippet } from 'svelte';
  import ChevronDown from '@lucide/svelte/icons/chevron-down';

  interface Option {
    value: T;
    label: string;
    disabled?: boolean;
  }
  interface Props {
    label: string;
    value: T;
    options: Option[];
    placeholder?: string;
    hint?: string;
    tag?: Snippet;
    below?: Snippet;
    id?: string;
    disabled?: boolean;
    hideLabel?: boolean;
    onchange?: (v: T) => void;
  }
  let {
    label,
    value = $bindable(),
    options,
    placeholder,
    hint,
    tag,
    below,
    id = `sel-${Math.random().toString(36).slice(2)}`,
    disabled = false,
    hideLabel = false,
    onchange,
  }: Props = $props();

  // A native <select> keeps keyboard and screen-reader behaviour right;
  // values are mapped through their index so numbers stay numbers.
  const selectedIndex = $derived(options.findIndex((o) => o.value === value));
</script>

<div class="field">
  <div class="label-row" class:sr-only={hideLabel}>
    <label for={id}>{label}</label>
    {#if tag}{@render tag()}{/if}
  </div>
  {#if hint}<p class="hint" id="{id}-hint">{hint}</p>{/if}
  <div class="control">
    <select
      {id}
      {disabled}
      aria-describedby={hint ? `${id}-hint` : undefined}
      value={selectedIndex >= 0 ? String(selectedIndex) : ''}
      onchange={(e) => {
        const i = Number((e.currentTarget as HTMLSelectElement).value);
        if (Number.isNaN(i) || !options[i]) return;
        value = options[i].value;
        onchange?.(options[i].value);
      }}
    >
      {#if placeholder}<option value="" disabled>{placeholder}</option>{/if}
      {#each options as o, i (i)}
        <option value={String(i)} disabled={o.disabled}>{o.label}</option>
      {/each}
    </select>
    <span class="chevron"><ChevronDown size={18} aria-hidden={true} /></span>
  </div>
  {#if below}{@render below()}{/if}
</div>

<style>
  .field {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    min-width: 0;
  }
  .label-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  label {
    font-weight: 600;
    font-size: 0.9375rem;
  }
  .hint {
    font-size: 0.875rem;
    color: var(--text-secondary);
    margin-top: -0.15rem;
  }
  .control {
    position: relative;
  }
  select {
    appearance: none;
    width: 100%;
    min-height: 46px;
    padding: 0.6rem 2.5rem 0.6rem 0.85rem;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--bg-raised);
    cursor: pointer;
    text-overflow: ellipsis;
  }
  select:hover:not(:disabled) {
    border-color: var(--text-secondary);
  }
  select:focus-visible {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 3px rgba(131, 17, 79, 0.18);
  }
  select:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
  .chevron {
    position: absolute;
    right: 0.75rem;
    top: 50%;
    transform: translateY(-50%);
    pointer-events: none;
    color: var(--text-secondary);
    display: flex;
  }
</style>
