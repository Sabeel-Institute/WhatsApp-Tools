<script lang="ts">
  import { onMount } from 'svelte';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import ChevronRight from '@lucide/svelte/icons/chevron-right';
  import UsersRound from '@lucide/svelte/icons/users-round';
  import ProgressBar from '../../components/ProgressBar.svelte';
  import Banner from '../../components/Banner.svelte';
  import Button from '../../components/Button.svelte';
  import Select from '../../components/Select.svelte';
  import Spinner from '../../components/Spinner.svelte';
  import TextField from '../../components/TextField.svelte';
  import Toggle from '../../components/Toggle.svelte';
  import { app } from '../../lib/state.svelte';
  import { num, people } from '../../lib/format';
  import type { Draft } from './draft.svelte';

  let { draft }: { draft: Draft } = $props();

  onMount(() => draft.loadCommunities());

  const isCreate = $derived(draft.mode === 'create');
  const communities = $derived(isCreate ? draft.adminCommunities : draft.addableCommunities);
  const max = $derived(app.info?.maxGroupSize ?? 1024);
  const adding = $derived(draft.list.selected.length);
</script>

<div class="step">
  <div class="head">
    <h2>{isCreate ? 'Group details' : 'Choose the group'}</h2>
    <p class="muted">
      {#if isCreate}
        Choose the Community for the new group and give it a name people will recognise.
      {:else}
        Choose the group to add {people(adding)} to.
      {/if}
    </p>
  </div>

  {#if draft.loadingCommunities && !draft.communities}
    <div class="loading" role="status">
      <Spinner size={22} /> <span class="muted">Loading your Communities from WhatsApp…</span>
    </div>
  {:else if draft.communitiesError}
    <Banner tone="danger" title="We couldn't load your Communities" live>
      {draft.communitiesError}
      {#snippet actions()}
        <Button
          size="sm"
          icon={RefreshCw}
          loading={draft.loadingCommunities}
          onclick={() => draft.loadCommunities(true)}>Try again</Button
        >
      {/snippet}
    </Banner>
  {:else if draft.communities && communities.length === 0}
    <Banner
      tone="warning"
      title={isCreate
        ? "This account can't create groups in any Community"
        : "This account isn't an admin of any group"}
    >
      {#if isCreate}
        To create a group inside a Community, the connected WhatsApp account must be an admin of that Community. Ask a
        Community admin to open the Community in WhatsApp, tap <strong>Members</strong>, choose this number ({app.status
          .account?.display}), and tap <strong>Make community admin</strong>. Then choose Refresh.
      {:else}
        To add people to a group, the connected WhatsApp account must be an admin of that group. Ask a group admin to
        make this number ({app.status.account?.display}) an admin, then choose Refresh.
      {/if}
      {#snippet actions()}
        <Button
          size="sm"
          icon={RefreshCw}
          loading={draft.loadingCommunities}
          onclick={() => draft.loadCommunities(true)}>Refresh</Button
        >
      {/snippet}
    </Banner>
  {:else if draft.communities}
    <div class="layout">
      <div class="fields">
        <div class="with-action">
          <Select
            label="Community"
            hint={isCreate ? 'Only Communities where this account is an admin are listed.' : undefined}
            bind:value={draft.communityId}
            placeholder="Choose a Community…"
            options={communities.map((c) => ({ value: c.id, label: c.name }))}
            onchange={() => (draft.groupId = '')}
          />
          <Button
            variant="quiet"
            size="sm"
            icon={RefreshCw}
            loading={draft.loadingCommunities}
            onclick={() => draft.loadCommunities(true)}>Refresh</Button
          >
        </div>

        {#if isCreate}
          <TextField
            label="Group name"
            hint="For example: Tafsir Circle Fall 2026"
            bind:value={draft.groupName}
            limit={app.info?.maxNameLength ?? 25}
          />
          <TextField
            label="Description"
            optional
            multiline
            rows={3}
            hint="Shown in the group's info. You can change it later in WhatsApp."
            bind:value={draft.description}
            limit={app.info?.maxDescLength ?? 2048}
          />

          <details class="more">
            <summary><ChevronRight size={18} aria-hidden={true} class="chev" /> More options</summary>
            <div class="toggles">
              <Toggle
                label="Only admins can send messages"
                description="Good for announcement-style groups. Everyone can still read."
                bind:checked={draft.settings.onlyAdminsSend}
              />
              <Toggle
                label="Only admins can edit group info"
                description="Stops members changing the group's name, picture and description."
                bind:checked={draft.settings.onlyAdminsEdit}
              />
              <Toggle
                label="New members need admin approval to join via link"
                description="People who use the invite link wait until an admin approves them."
                bind:checked={draft.settings.approvalRequired}
              />
            </div>
          </details>
        {:else if draft.community}
          {@const groups = draft.community.groups}
          <Select
            label="Group"
            bind:value={draft.groupId}
            placeholder={groups.length ? 'Choose a group…' : 'No groups yet'}
            options={groups.map((g) => ({
              value: g.id,
              label: `${g.name} · ${num(g.size)} members${g.isAdmin ? '' : ' (you are not an admin)'}`,
              disabled: !g.isAdmin,
            }))}
          />
        {/if}

        {#if draft.overLimit}
          <Banner tone="warning" title="That's more than a WhatsApp group can hold">
            A WhatsApp group can have at most {num(max)} members, including you.
            {#if isCreate}
              This list has {people(adding)}.
            {:else}
              “{draft.group?.name}” already has {num(draft.group?.size ?? 0)} members, and you're adding {people(
                adding,
              )}.
            {/if}
            Go back to the review step and untick
            {num(draft.finalSize - max)}
            {draft.finalSize - max === 1 ? 'person' : 'people'}, or split the list into two groups.
          </Banner>
        {/if}
      </div>
      <aside class="aside" aria-label="Summary">
        <span class="aicon"><UsersRound size={22} aria-hidden={true} /></span>
        <p class="anum tabular">{num(adding)}</p>
        <p class="alabel">{adding === 1 ? 'person' : 'people'} to add</p>
        {#if !isCreate && !draft.group}
          <p class="anote">Choose a group to see how big it will be.</p>
        {:else}
          <ProgressBar
            value={draft.finalSize / max}
            label="Group size"
            size="sm"
            tone={draft.overLimit ? 'warning' : 'success'}
          />
          <p class="anote">
            The group will have {num(draft.finalSize)} members{isCreate ? ', including you' : ''}. WhatsApp allows up to {num(
              max,
            )}.
          </p>
        {/if}
      </aside>
    </div>
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
  .loading {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 1.5rem 0;
  }
  .layout {
    display: grid;
    grid-template-columns: minmax(0, 620px) 240px;
    justify-content: space-between;
    gap: 2rem;
    align-items: start;
  }
  .fields {
    display: flex;
    flex-direction: column;
    gap: 1.35rem;
  }
  .aside {
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg-raised);
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }
  .aicon {
    width: 40px;
    height: 40px;
    border-radius: 12px;
    display: grid;
    place-items: center;
    background: var(--bg-accent-soft);
    color: var(--accent);
    margin-bottom: 0.4rem;
  }
  .anum {
    font-size: 2rem;
    font-weight: 700;
    line-height: 1.1;
  }
  .alabel {
    font-weight: 600;
    margin-bottom: 0.6rem;
  }
  .anote {
    font-size: 0.875rem;
    color: var(--text-secondary);
    margin-top: 0.4rem;
  }
  @media (max-width: 900px) {
    .layout {
      grid-template-columns: 1fr;
    }
  }
  .with-action {
    display: grid;
    grid-template-columns: 1fr auto;
    align-items: end;
    gap: 0.5rem;
  }
  .with-action :global(button) {
    margin-bottom: 6px;
  }
  .more {
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg-raised);
  }
  .more summary {
    list-style: none;
    cursor: pointer;
    padding: 0.85rem 1rem;
    font-weight: 600;
    display: flex;
    align-items: center;
    gap: 0.4rem;
    border-radius: var(--radius);
  }
  .more summary::-webkit-details-marker {
    display: none;
  }
  .more summary:hover {
    background: var(--bg-hover);
  }
  .more :global(.chev) {
    transition: transform 0.15s ease;
    color: var(--text-secondary);
  }
  .more[open] :global(.chev) {
    transform: rotate(90deg);
  }
  .toggles {
    padding: 0 1rem 0.4rem;
    border-top: 1px solid var(--border-subtle);
  }
</style>
