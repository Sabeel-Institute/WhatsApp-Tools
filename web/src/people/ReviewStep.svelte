<script lang="ts">
  import { onMount } from 'svelte';
  import CircleCheck from '@lucide/svelte/icons/circle-check';
  import WandSparkles from '@lucide/svelte/icons/wand-sparkles';
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
  import Copy from '@lucide/svelte/icons/copy';
  import MessageCircleOff from '@lucide/svelte/icons/message-circle-off';
  import UserCheck from '@lucide/svelte/icons/user-check';
  import Pencil from '@lucide/svelte/icons/pencil';
  import Check from '@lucide/svelte/icons/check';
  import X from '@lucide/svelte/icons/x';
  import Banner from '../components/Banner.svelte';
  import Button from '../components/Button.svelte';
  import Pill from '../components/Pill.svelte';
  import ProgressBar from '../components/ProgressBar.svelte';
  import Spinner from '../components/Spinner.svelte';
  import { app } from '../lib/state.svelte';
  import { toast } from '../components/toaster.svelte';
  import { num, people as peopleCount } from '../lib/format';
  import type { Person, PersonStatus } from '../lib/types';
  import { usable, type PeopleList } from './list.svelte';

  let { list }: { list: PeopleList } = $props();

  type Filter = 'all' | PersonStatus;
  let filter = $state<Filter>('all');
  let limit = $state(150);
  let editing = $state<number | null>(null);
  let draft = $state('');
  let saving = $state(false);

  onMount(async () => {
    await list.runReview();
    await list.checkWhatsApp();
  });

  // When the WhatsApp check finishes, refresh the review with its answers.
  let watched: string | null = null;
  $effect(() => {
    const job = list.checkJob;
    if (!job) return;
    const key = `${job.id}:${job.state}`;
    if (key === watched) return;
    watched = key;
    if (job.state === 'completed') list.runReview();
    else if (job.state === 'failed') list.checkError = job.error ?? "We couldn't check the numbers with WhatsApp.";
  });

  const review = $derived(list.review);
  const counts = $derived(review?.counts);
  const checkJob = $derived(list.checkJob);
  const checking = $derived(!!checkJob && !['completed', 'failed', 'stopped'].includes(checkJob.state));

  const statusView: Record<
    PersonStatus,
    { tone: 'success' | 'info' | 'warning' | 'neutral' | 'danger'; text: string; icon: typeof CircleCheck }
  > = {
    ready: { tone: 'success', text: 'Ready', icon: CircleCheck },
    fixed: { tone: 'info', text: 'Fixed automatically', icon: WandSparkles },
    attention: { tone: 'warning', text: 'Needs attention', icon: TriangleAlert },
    duplicate: { tone: 'neutral', text: 'Duplicate', icon: Copy },
    not_on_whatsapp: { tone: 'danger', text: 'Not on WhatsApp', icon: MessageCircleOff },
    self: { tone: 'neutral', text: 'This is you', icon: UserCheck },
  };

  const chips = $derived(
    counts
      ? ([
          { key: 'all', label: 'Everyone', n: counts.total, tone: 'all' },
          { key: 'ready', label: 'Ready', n: counts.ready - counts.fixed, tone: 'success' },
          { key: 'fixed', label: 'Fixed automatically', n: counts.fixed, tone: 'info' },
          { key: 'attention', label: 'Needs attention', n: counts.attention, tone: 'warning' },
          { key: 'duplicate', label: 'Duplicates removed', n: counts.duplicates, tone: 'neutral' },
          { key: 'not_on_whatsapp', label: 'Not on WhatsApp', n: counts.notOnWhatsApp, tone: 'danger' },
        ] as const)
      : [],
  );

  const visible = $derived((review?.people ?? []).filter((p) => filter === 'all' || p.status === filter));
  const shown = $derived(visible.slice(0, limit));
  const selectable = $derived(visible.filter((p) => usable(p) && p.onWhatsApp !== false));
  const allChecked = $derived(selectable.length > 0 && selectable.every((p) => !list.excluded[p.row]));

  function toggleAll() {
    const to = !allChecked;
    for (const p of selectable) list.excluded[p.row] = !to;
  }

  function startEdit(p: Person) {
    editing = p.row;
    draft = list.edits[p.row] ?? p.raw;
  }

  async function saveEdit(row: number) {
    saving = true;
    await list.editPhone(row, draft);
    saving = false;
    editing = null;
    const p = list.review?.people.find((x) => x.row === row);
    if (p) {
      const who = p.name || `Row ${p.row}`;
      toast.show(usable(p) ? `Number updated: ${who} is ready` : `Number updated: ${who} still needs attention`);
    }
  }
</script>

<div class="step">
  <div class="head">
    <h2>Review the list</h2>
    <p class="muted">
      We've tidied up every phone number. Untick anyone you don't want to add, and fix numbers that need attention.
    </p>
  </div>

  {#if list.reviewError}
    <Banner tone="danger" title="We couldn't review the list" live>{list.reviewError}</Banner>
  {/if}

  {#if !review}
    <div class="loading"><Spinner size={22} /> <span class="muted">Checking every number…</span></div>
  {:else}
    <div class="chips" role="group" aria-label="Show">
      {#each chips as c (c.key)}
        {#if c.key === 'all' || c.n > 0}
          <button
            class="chip {c.tone}"
            class:active={filter === c.key}
            aria-pressed={filter === c.key}
            onclick={() => {
              filter = c.key;
              limit = 150;
            }}
          >
            <span class="chip-n tabular">{num(c.n)}</span>
            <span>{c.label}</span>
          </button>
        {/if}
      {/each}
    </div>

    {#if checking && checkJob}
      <div class="checking" role="status">
        <div class="checking-text">
          <Spinner size={18} />
          <span>Checking which numbers have WhatsApp… {num(checkJob.done)} of {num(checkJob.total)}</span>
        </div>
        <ProgressBar value={checkJob.total ? checkJob.done / checkJob.total : 0} label="Checking numbers" size="sm" />
      </div>
    {:else if list.checkError}
      <Banner tone="warning" title="We couldn't check the numbers with WhatsApp yet">
        {list.checkError} You can continue anyway; numbers are checked again when people are added.
        {#snippet actions()}
          <Button size="sm" onclick={() => list.checkWhatsApp()}>Try again</Button>
        {/snippet}
      </Banner>
    {:else if !app.connected && counts && counts.unchecked > 0}
      <Banner tone="info">
        WhatsApp isn't connected right now, so we haven't checked which numbers have WhatsApp. That happens
        automatically when people are added.
      </Banner>
    {/if}

    {#if counts && counts.attention > 0 && filter !== 'attention'}
      <Banner tone="warning">
        {peopleCount(counts.attention)}
        {counts.attention === 1 ? 'has a number that needs' : 'have numbers that need'} attention and won't be added unless
        you fix {counts.attention === 1 ? 'it' : 'them'}.
        <button class="link" onclick={() => (filter = 'attention')}
          >Show {counts.attention === 1 ? 'it' : 'them'}</button
        >
      </Banner>
    {/if}

    <div class="list" role="table" aria-label="People" aria-busy={list.reviewing}>
      <div class="row header" role="row">
        <span role="columnheader" class="c-check">
          <input
            type="checkbox"
            checked={allChecked}
            disabled={selectable.length === 0}
            aria-label={allChecked ? 'Untick everyone shown' : 'Tick everyone shown'}
            onchange={toggleAll}
          />
        </span>
        <span role="columnheader">Name</span>
        <span role="columnheader">Phone</span>
        <span role="columnheader">Status</span>
      </div>
      {#each shown as p (p.row)}
        {@const sv = statusView[p.status]}
        {@const canInclude = usable(p) && p.onWhatsApp !== false}
        {@const included = list.isIncluded(p)}
        <div class="row" class:dim={!included} role="row">
          <span role="cell" class="c-check">
            <input
              type="checkbox"
              checked={included}
              disabled={!canInclude}
              aria-label="Add {p.name || p.display || `row ${p.row}`}"
              onchange={(e) => (list.excluded[p.row] = !(e.currentTarget as HTMLInputElement).checked)}
            />
          </span>
          <span role="cell" class="c-name">
            <span class="name" class:noname={!p.name}>{p.name || 'No name'}</span>
            <span class="rownum">Row {p.row}</span>
          </span>
          <span role="cell" class="c-phone">
            {#if editing === p.row}
              <form
                class="edit"
                onsubmit={(e) => {
                  e.preventDefault();
                  saveEdit(p.row);
                }}
              >
                <!-- svelte-ignore a11y_autofocus -->
                <input
                  class="edit-input"
                  bind:value={draft}
                  inputmode="tel"
                  aria-label="Phone number for {p.name || `row ${p.row}`}"
                  autofocus
                  onkeydown={(e) => e.key === 'Escape' && (editing = null)}
                />
                <Button
                  type="submit"
                  size="sm"
                  variant="primary"
                  icon={Check}
                  loading={saving}
                  ariaLabel="Save number"
                />
                <Button size="sm" variant="quiet" icon={X} ariaLabel="Cancel" onclick={() => (editing = null)} />
              </form>
            {:else}
              <span class="phone tabular" class:raw={!p.display}>{p.display || p.raw || '—'}</span>
              {#if p.status === 'attention'}
                <button
                  class="edit-btn fix"
                  onclick={() => startEdit(p)}
                  aria-label="Edit the number for {p.name || `row ${p.row}`}"
                >
                  <Pencil size={15} aria-hidden={true} />
                  <span>Fix</span>
                </button>
              {:else}
                <button
                  class="edit-btn quiet"
                  onclick={() => startEdit(p)}
                  aria-label="Edit the number for {p.name || `row ${p.row}`}"
                  title="Edit this number"
                >
                  <Pencil size={15} aria-hidden={true} />
                </button>
              {/if}
            {/if}
          </span>
          <span role="cell" class="c-status">
            {#if usable(p) && p.onWhatsApp === null && checking}
              <Pill tone="info" text="Checking…" />
            {:else}
              <Pill tone={sv.tone} icon={sv.icon} text={sv.text} />
            {/if}
            {#if p.note}<span class="note">{p.note}</span>{/if}
            {#if p.alsoRows?.length}<span class="note"
                >Also in row{p.alsoRows.length > 1 ? 's' : ''} {p.alsoRows.join(', ')}.</span
              >{/if}
          </span>
        </div>
      {:else}
        <p class="empty">Nobody here.</p>
      {/each}
    </div>
    {#if visible.length > limit}
      <div class="more">
        <Button onclick={() => (limit += 300)}>Show more ({num(visible.length - limit)} not shown)</Button>
      </div>
    {/if}
  {/if}
</div>

<style>
  .step {
    display: flex;
    flex-direction: column;
    gap: 1.1rem;
  }
  .head p {
    margin-top: 0.35rem;
  }
  .loading {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 2rem 0;
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.45rem 0.85rem 0.45rem 0.5rem;
    border-radius: 999px;
    border: 1px solid var(--border);
    background: var(--bg-raised);
    cursor: pointer;
    font-size: 0.9375rem;
    font-weight: 560;
    color: var(--text-primary);
    transition:
      border-color 0.15s ease,
      background-color 0.15s ease;
  }
  .chip:hover {
    border-color: var(--border-strong);
  }
  .chip-n {
    min-width: 28px;
    height: 24px;
    padding: 0 0.45rem;
    border-radius: 999px;
    display: inline-grid;
    place-items: center;
    font-weight: 700;
    font-size: 0.8125rem;
    background: var(--neutral-soft);
    color: var(--text-secondary);
  }
  .chip.success .chip-n {
    background: var(--success-soft);
    color: var(--success-text);
  }
  .chip.info .chip-n {
    background: var(--info-soft);
    color: var(--info);
  }
  .chip.warning .chip-n {
    background: var(--warning-soft);
    color: var(--warning-text);
  }
  .chip.danger .chip-n {
    background: var(--danger-soft);
    color: var(--danger);
  }
  .chip.active {
    border-color: var(--accent);
    box-shadow: inset 0 0 0 1px var(--accent);
  }
  .checking {
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
    padding: 0.85rem 1rem;
    border-radius: var(--radius);
    background: var(--info-soft);
    border: 1px solid #c9dbe6;
  }
  .checking-text {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    color: var(--info);
    font-weight: 560;
    font-size: 0.9375rem;
  }
  .link {
    background: none;
    border: none;
    padding: 0;
    color: var(--text-accent);
    text-decoration: underline;
    text-underline-offset: 3px;
    cursor: pointer;
    font-weight: 600;
  }
  .list {
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg-raised);
    overflow: hidden;
  }
  .row {
    display: grid;
    grid-template-columns: 48px minmax(150px, 1.1fr) minmax(200px, 1fr) minmax(220px, 1.3fr);
    gap: 0.75rem;
    align-items: center;
    padding: 0.8rem 1rem 0.8rem 0.5rem;
  }
  .row + .row {
    border-top: 1px solid var(--border-subtle);
  }
  .row.header {
    background: var(--bg-inset);
    font-size: 0.8125rem;
    font-weight: 650;
    color: var(--text-secondary);
    padding-top: 0.55rem;
    padding-bottom: 0.55rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .row.dim .c-name,
  .row.dim .phone {
    color: var(--text-secondary);
  }
  .c-check {
    display: flex;
    justify-content: center;
  }
  .c-check input {
    width: 20px;
    height: 20px;
    accent-color: var(--accent);
    cursor: pointer;
  }
  .c-check input:disabled {
    cursor: not-allowed;
    opacity: 0.45;
  }
  .c-name {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .name {
    font-weight: 580;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .noname {
    font-weight: 450;
    font-style: italic;
    color: var(--text-secondary);
  }
  .rownum {
    font-size: 0.8125rem;
    color: var(--text-secondary);
  }
  .c-phone {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    min-width: 0;
  }
  .phone {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .phone.raw {
    color: var(--text-secondary);
  }
  .edit-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    border: none;
    background: none;
    color: var(--text-accent);
    font-size: 0.8125rem;
    font-weight: 600;
    padding: 0.3rem 0.5rem;
    border-radius: 6px;
    cursor: pointer;
    opacity: 0.85;
    flex: none;
  }
  .edit-btn:hover {
    background: var(--bg-accent-soft);
    opacity: 1;
  }
  .edit-btn.quiet {
    color: var(--text-secondary);
    opacity: 0.55;
  }
  .row:hover .edit-btn.quiet,
  .edit-btn.quiet:focus-visible {
    opacity: 1;
    color: var(--text-accent);
  }
  .edit-btn.fix {
    border: 1px solid #e6d3a8;
    background: var(--warning-soft);
    color: var(--warning-text);
    opacity: 1;
  }
  .edit {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    width: 100%;
  }
  .edit-input {
    flex: 1;
    min-width: 0;
    border: 1px solid var(--accent);
    border-radius: 8px;
    padding: 0.4rem 0.6rem;
    background: #fff;
    box-shadow: 0 0 0 3px rgba(131, 17, 79, 0.15);
  }
  .edit-input:focus-visible {
    outline: none;
  }
  .c-status {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 0.25rem;
    min-width: 0;
  }
  .note {
    font-size: 0.8125rem;
    color: var(--text-secondary);
    line-height: 1.4;
  }
  .empty {
    padding: 1.5rem;
    text-align: center;
    color: var(--text-secondary);
  }
  .more {
    display: flex;
    justify-content: center;
  }
  @media (max-width: 860px) {
    .row {
      grid-template-columns: 40px 1fr;
      row-gap: 0.4rem;
    }
    .row.header span:nth-child(n + 3) {
      display: none;
    }
    .c-phone,
    .c-status {
      grid-column: 2;
    }
  }
</style>
