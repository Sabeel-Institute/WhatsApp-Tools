<script lang="ts">
  interface Props {
    label: string;
    description?: string;
    checked: boolean;
    id?: string;
  }
  let { label, description, checked = $bindable(), id = `tg-${Math.random().toString(36).slice(2)}` }: Props = $props();
</script>

<div class="toggle">
  <div class="text">
    <label for={id}>{label}</label>
    {#if description}<p id="{id}-desc">{description}</p>{/if}
  </div>
  <button
    {id}
    type="button"
    role="switch"
    aria-checked={checked}
    aria-describedby={description ? `${id}-desc` : undefined}
    onclick={() => (checked = !checked)}
  >
    <span class="knob"></span>
    <span class="state" aria-hidden="true">{checked ? 'On' : 'Off'}</span>
  </button>
</div>

<style>
  .toggle {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1.25rem;
    padding: 0.85rem 0;
  }
  .toggle + :global(.toggle) {
    border-top: 1px solid var(--border-subtle);
  }
  label {
    font-weight: 560;
    cursor: pointer;
  }
  p {
    font-size: 0.875rem;
    color: var(--text-secondary);
  }
  button {
    position: relative;
    flex: none;
    width: 76px;
    height: 34px;
    border-radius: 999px;
    border: 1px solid var(--border-strong);
    background: var(--bg-inset);
    cursor: pointer;
    transition: background-color 0.15s ease;
    padding: 0;
  }
  button:focus-visible {
    outline: none;
    box-shadow: var(--focus);
  }
  .knob {
    position: absolute;
    top: 3px;
    left: 3px;
    width: 26px;
    height: 26px;
    border-radius: 50%;
    background: var(--bg-raised);
    box-shadow: 0 1px 3px rgba(58, 47, 40, 0.25);
    transition: transform 0.18s ease;
  }
  .state {
    position: absolute;
    top: 50%;
    transform: translateY(-50%);
    right: 12px;
    font-size: 0.75rem;
    font-weight: 650;
    color: var(--text-secondary);
  }
  [aria-checked='true'] {
    background: var(--accent);
    border-color: var(--accent);
  }
  [aria-checked='true'] .knob {
    transform: translateX(42px);
  }
  [aria-checked='true'] .state {
    right: auto;
    left: 12px;
    color: var(--on-accent);
  }
</style>
