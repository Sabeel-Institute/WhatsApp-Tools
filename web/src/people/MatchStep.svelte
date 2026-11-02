<script lang="ts">
  import { onMount } from 'svelte';
  import Phone from '@lucide/svelte/icons/phone';
  import User from '@lucide/svelte/icons/user';
  import Globe from '@lucide/svelte/icons/globe';
  import Banner from '../components/Banner.svelte';
  import Select from '../components/Select.svelte';
  import ColumnField from './ColumnField.svelte';
  import { api } from '../lib/api';
  import type { Country, NameMode } from '../lib/types';
  import type { PeopleList } from './list.svelte';

  let { list }: { list: PeopleList } = $props();
  let countries = $state<Country[]>([]);

  onMount(() => {
    api
      .get<Country[]>('/api/countries')
      .then((c) => (countries = c))
      .catch(() => {});
  });

  const columns = $derived(list.table?.columns ?? []);
  const modes: { value: NameMode; label: string }[] = [
    { value: 'full', label: 'One column with the full name' },
    { value: 'split', label: 'First and last name in separate columns' },
    { value: 'none', label: "Don't use names" },
  ];
</script>

<div class="step">
  <div class="head">
    <h2>Match your columns</h2>
    <p class="muted">
      WhatsApp only needs a phone number. Adding names helps you review the list and the results. We've made a
      suggestion; check it and change anything that's wrong.
    </p>
  </div>

  {#if list.remembered}
    <Banner tone="success">We've used the same choices as last time you imported a file with these columns.</Banner>
  {/if}

  <section class="group">
    <div class="group-head">
      <span class="gicon"><Phone size={18} aria-hidden={true} /></span>
      <h3>Phone number <span class="req">required</span></h3>
    </div>
    <ColumnField
      label="Which column has the phone number?"
      bind:value={list.mapping.phone}
      {columns}
      suggested={list.suggested.phone}
      onchange={() => (list.suggested.phone = false)}
    />
    {#if list.mapping.phone < 0}
      <p class="none-found">We couldn't tell which column has the phone numbers. Please choose it.</p>
    {/if}
  </section>

  <section class="group">
    <div class="group-head">
      <span class="gicon"><User size={18} aria-hidden={true} /></span>
      <h3>Name <span class="opt">optional</span></h3>
    </div>
    <fieldset class="modes">
      <legend class="sr-only">How are names written in this file?</legend>
      {#each modes as m (m.value)}
        <label class="mode" class:checked={list.mapping.nameMode === m.value}>
          <input
            type="radio"
            name="name-mode"
            value={m.value}
            checked={list.mapping.nameMode === m.value}
            onchange={() => (list.mapping.nameMode = m.value)}
          />
          <span>{m.label}</span>
        </label>
      {/each}
    </fieldset>

    {#if list.mapping.nameMode === 'full'}
      <ColumnField
        label="Which column has the name?"
        bind:value={list.mapping.name}
        {columns}
        suggested={list.suggested.name}
        onchange={() => (list.suggested.name = false)}
      />
    {:else if list.mapping.nameMode === 'split'}
      <div class="two">
        <ColumnField
          label="First name column"
          bind:value={list.mapping.first}
          {columns}
          suggested={list.suggested.first}
          onchange={() => (list.suggested.first = false)}
        />
        <ColumnField
          label="Last name column"
          bind:value={list.mapping.last}
          {columns}
          suggested={list.suggested.last}
          onchange={() => (list.suggested.last = false)}
        />
      </div>
    {/if}
  </section>

  <section class="group">
    <div class="group-head">
      <span class="gicon"><Globe size={18} aria-hidden={true} /></span>
      <h3>Country</h3>
    </div>
    <Select
      label="Country for numbers written without a country code"
      hint="Numbers that start with + and a country code are read correctly whatever you choose here."
      bind:value={list.country}
      options={countries.length
        ? countries.map((c) => ({ value: c.code, label: `${c.name} (${c.dial})` }))
        : [{ value: list.country, label: list.country === 'US' ? 'United States (+1)' : list.country }]}
    />
  </section>
</div>

<style>
  .step {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .head p {
    margin-top: 0.35rem;
    max-width: 660px;
  }
  .group {
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg-raised);
    padding: 1.25rem 1.35rem 1.35rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }
  .group-head {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }
  .gicon {
    width: 32px;
    height: 32px;
    border-radius: 9px;
    display: grid;
    place-items: center;
    background: var(--bg-accent-soft);
    color: var(--accent);
  }
  .req,
  .opt {
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin-left: 0.4rem;
    color: var(--text-secondary);
  }
  .req {
    color: var(--text-accent);
  }
  .none-found {
    color: var(--warning-text);
    font-size: 0.9375rem;
  }
  .modes {
    border: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }
  .mode {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.55rem 0.9rem;
    border: 1px solid var(--border-strong);
    border-radius: 999px;
    cursor: pointer;
    font-size: 0.9375rem;
    background: var(--bg-raised);
    transition:
      background-color 0.15s ease,
      border-color 0.15s ease;
  }
  .mode:hover {
    border-color: var(--text-secondary);
  }
  .mode.checked {
    border-color: var(--accent);
    background: var(--bg-accent-soft);
    color: var(--accent);
    font-weight: 600;
  }
  .mode input {
    accent-color: var(--accent);
    margin: 0;
  }
  .mode:has(input:focus-visible) {
    box-shadow: var(--focus);
  }
  .two {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
  }
  @media (max-width: 700px) {
    .two {
      grid-template-columns: 1fr;
    }
  }
</style>
