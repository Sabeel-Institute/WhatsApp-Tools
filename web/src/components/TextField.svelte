<script lang="ts">
  interface Props {
    label: string;
    value: string;
    hint?: string;
    placeholder?: string;
    error?: string | null;
    /** Shows a live counter and marks the field when over the limit. */
    limit?: number;
    multiline?: boolean;
    rows?: number;
    optional?: boolean;
    inputmode?: 'text' | 'tel' | 'numeric';
    autocomplete?: 'off' | 'tel' | 'name';
    id?: string;
    autofocus?: boolean;
    onkeydown?: (e: KeyboardEvent) => void;
  }
  let {
    label,
    value = $bindable(),
    hint,
    placeholder,
    error = null,
    limit,
    multiline = false,
    rows = 3,
    optional = false,
    inputmode = 'text',
    autocomplete = 'off',
    id = `tf-${Math.random().toString(36).slice(2)}`,
    autofocus = false,
    onkeydown,
  }: Props = $props();

  const length = $derived(value.length);
  const over = $derived(limit !== undefined && length > limit);
  const describedBy = $derived(
    [hint && `${id}-hint`, (error || over) && `${id}-err`, limit && `${id}-count`].filter(Boolean).join(' '),
  );
  let el: HTMLInputElement | HTMLTextAreaElement | undefined = $state();
  $effect(() => {
    if (autofocus) el?.focus();
  });
</script>

<div class="field" class:invalid={!!error || over}>
  <div class="label-row">
    <label for={id}
      >{label}{#if optional}<span class="optional">(optional)</span>{/if}</label
    >
    {#if limit !== undefined}
      <span class="count tabular" class:over id="{id}-count" aria-live="polite">
        {length} / {limit}<span class="sr-only"> characters</span>
      </span>
    {/if}
  </div>
  {#if hint}<p class="hint" id="{id}-hint">{hint}</p>{/if}
  {#if multiline}
    <textarea
      bind:this={el}
      {id}
      {rows}
      {placeholder}
      bind:value
      aria-invalid={!!error || over}
      aria-describedby={describedBy || undefined}
      {onkeydown}></textarea>
  {:else}
    <input
      bind:this={el}
      {id}
      type="text"
      {placeholder}
      {inputmode}
      {autocomplete}
      spellcheck="false"
      bind:value
      aria-invalid={!!error || over}
      aria-describedby={describedBy || undefined}
      {onkeydown}
    />
  {/if}
  {#if error}
    <p class="error" id="{id}-err">{error}</p>
  {:else if over && limit !== undefined}
    <p class="error" id="{id}-err">
      That's {length - limit} character{length - limit === 1 ? '' : 's'} too long. WhatsApp allows up to {limit}.
    </p>
  {/if}
</div>

<style>
  .field {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }
  .label-row {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    gap: 1rem;
  }
  label {
    font-weight: 600;
    font-size: 0.9375rem;
  }
  .optional {
    margin-left: 0.35rem;
    font-weight: 450;
    color: var(--text-secondary);
  }
  .hint {
    font-size: 0.875rem;
    color: var(--text-secondary);
    margin-top: -0.15rem;
  }
  .count {
    font-size: 0.8125rem;
    color: var(--text-secondary);
  }
  .count.over {
    color: var(--danger);
    font-weight: 600;
  }
  input,
  textarea {
    width: 100%;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--bg-raised);
    padding: 0.65rem 0.85rem;
    min-height: 46px;
    transition:
      border-color 0.15s ease,
      box-shadow 0.15s ease;
  }
  textarea {
    resize: vertical;
    line-height: 1.5;
  }
  input::placeholder,
  textarea::placeholder {
    color: var(--text-muted);
  }
  input:hover,
  textarea:hover {
    border-color: var(--text-secondary);
  }
  input:focus-visible,
  textarea:focus-visible {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 3px rgba(131, 17, 79, 0.18);
  }
  .invalid input,
  .invalid textarea {
    border-color: var(--danger);
  }
  .error {
    color: var(--danger);
    font-size: 0.875rem;
  }
</style>
