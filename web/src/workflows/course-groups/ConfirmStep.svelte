<script lang="ts">
  import UsersRound from '@lucide/svelte/icons/users-round';
  import UserPlus from '@lucide/svelte/icons/user-plus';
  import ShieldCheck from '@lucide/svelte/icons/shield-check';
  import Clock from '@lucide/svelte/icons/clock';
  import Banner from '../../components/Banner.svelte';
  import { app } from '../../lib/state.svelte';
  import { duration, num, people } from '../../lib/format';
  import type { Draft } from './draft.svelte';

  let { draft }: { draft: Draft } = $props();

  const n = $derived(draft.list.selected.length);
  const counts = $derived(draft.list.review?.counts);
  const left = $derived((counts?.total ?? 0) - n);
  const pacing = $derived(app.info?.pacing);
  const seconds = $derived.by(() => {
    if (!pacing) return 0;
    const batches = Math.ceil(n / pacing.addBatchSize);
    const pause = (pacing.batchPauseMinSeconds + pacing.batchPauseMaxSeconds) / 2;
    return batches * 2 + Math.max(0, batches - 1) * pause + (draft.mode === 'create' ? 5 : 2);
  });
  const settingsText = $derived(
    [
      draft.settings.onlyAdminsSend ? 'only admins can send messages' : 'everyone can send messages',
      draft.settings.onlyAdminsEdit ? 'only admins can edit group info' : 'everyone can edit group info',
      draft.settings.approvalRequired ? 'joining by link needs approval' : 'anyone with the link can join',
    ].join(', '),
  );
</script>

<div class="step">
  <div class="head">
    <h2>Ready to {draft.mode === 'create' ? 'create the group' : 'add people'}</h2>
    <p class="muted">Check the details below, then start. You can pause or stop at any time.</p>
  </div>

  <div class="summary">
    <span class="sicon">
      {#if draft.mode === 'create'}<UsersRound size={28} aria-hidden={true} />{:else}<UserPlus
          size={28}
          aria-hidden={true}
        />{/if}
    </span>
    <p class="sentence">
      {#if draft.mode === 'create'}
        Create <strong>“{draft.groupName.trim()}”</strong> in <strong>“{draft.community?.name}”</strong> and add
        <strong>{people(n)}</strong>.
      {:else}
        Add <strong>{people(n)}</strong> to <strong>“{draft.group?.name}”</strong> in
        <strong>“{draft.community?.name}”</strong>.
      {/if}
    </p>
  </div>

  <dl class="facts">
    {#if draft.mode === 'create'}
      <div>
        <dt>Description</dt>
        <dd>{draft.description.trim() || 'None'}</dd>
      </div>
      <div>
        <dt>Settings</dt>
        <dd>{settingsText.charAt(0).toUpperCase() + settingsText.slice(1)}.</dd>
      </div>
    {:else}
      <div>
        <dt>Group size afterwards</dt>
        <dd>Up to {num(draft.finalSize)} members</dd>
      </div>
    {/if}
    <div>
      <dt>Left out</dt>
      <dd>
        {#if left > 0}
          {people(left)} from the file (numbers that need fixing, duplicates, people without WhatsApp, or people you unticked).
        {:else}
          Nobody. Everyone in the file will be added.
        {/if}
      </dd>
    </div>
  </dl>

  <div class="notes">
    <p class="note">
      <ShieldCheck size={18} aria-hidden={true} />
      To keep the WhatsApp account safe, people are added {pacing?.addBatchSize ?? 5} at a time with short pauses in between.
    </p>
    <p class="note">
      <Clock size={18} aria-hidden={true} />
      This should take {duration(seconds)}. Keep the app open until it finishes; everyone added so far stays added if
      you stop.
    </p>
  </div>

  {#if draft.startError}
    <Banner tone="danger" title="It didn't start" live>{draft.startError}</Banner>
  {/if}
</div>

<style>
  .step {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .head p {
    margin-top: 0.35rem;
  }
  .summary {
    display: flex;
    gap: 1.1rem;
    align-items: center;
    padding: 1.4rem 1.5rem;
    border-radius: var(--radius-lg);
    background: var(--bg-raised);
    border: 1px solid #dcc2c9;
    box-shadow: var(--shadow-sm);
  }
  .sicon {
    flex: none;
    width: 56px;
    height: 56px;
    border-radius: 16px;
    display: grid;
    place-items: center;
    background: var(--accent);
    color: var(--on-accent);
  }
  .sentence {
    font-size: 1.25rem;
    line-height: 1.45;
  }
  .sentence strong {
    font-weight: 650;
  }
  .facts {
    margin: 0;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg-raised);
  }
  .facts div {
    display: grid;
    grid-template-columns: 200px 1fr;
    gap: 1rem;
    padding: 0.85rem 1.25rem;
  }
  .facts div + div {
    border-top: 1px solid var(--border-subtle);
  }
  dt {
    font-weight: 600;
    color: var(--text-secondary);
  }
  dd {
    margin: 0;
    overflow-wrap: anywhere;
  }
  .notes {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .note {
    display: flex;
    gap: 0.6rem;
    align-items: flex-start;
    color: var(--text-secondary);
  }
  .note :global(svg) {
    flex: none;
    margin-top: 3px;
    color: var(--success);
  }
  @media (max-width: 640px) {
    .facts div {
      grid-template-columns: 1fr;
      gap: 0.2rem;
    }
  }
</style>
