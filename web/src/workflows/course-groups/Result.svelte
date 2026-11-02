<script lang="ts">
  import PartyPopper from '@lucide/svelte/icons/party-popper';
  import CircleStop from '@lucide/svelte/icons/circle-stop';
  import CircleAlert from '@lucide/svelte/icons/circle-alert';
  import Copy from '@lucide/svelte/icons/copy';
  import Download from '@lucide/svelte/icons/download';
  import House from '@lucide/svelte/icons/house';
  import Send from '@lucide/svelte/icons/send';
  import UserPlus from '@lucide/svelte/icons/user-plus';
  import Link2 from '@lucide/svelte/icons/link-2';
  import Banner from '../../components/Banner.svelte';
  import Button from '../../components/Button.svelte';
  import Modal from '../../components/Modal.svelte';
  import Pill from '../../components/Pill.svelte';
  import ProgressBar from '../../components/ProgressBar.svelte';
  import Spinner from '../../components/Spinner.svelte';
  import { copyText } from '../../components/toaster.svelte';
  import { api, ApiError, download } from '../../lib/api';
  import { app, router } from '../../lib/state.svelte';
  import { duration, num, people, when } from '../../lib/format';
  import { jobFinished, type Job, type Run } from '../../lib/types';
  import Progress from './Progress.svelte';
  import { outcomeOf, type CGDetail } from './outcomes';

  let { run }: { run: Run<CGDetail> } = $props();

  const d = $derived(run.detail!);
  const jobsForRun = $derived(Object.values(app.jobs).filter((j) => j.runId === run.id));
  const mainJob = $derived(jobsForRun.find((j) => (j.kind === 'create' || j.kind === 'add') && !jobFinished(j.state)));
  const inviteJob = $derived(
    jobsForRun.filter((j) => j.kind === 'invites').sort((a, b) => b.startedAt.localeCompare(a.startedAt))[0],
  );
  const invitesActive = $derived(!!inviteJob && !jobFinished(inviteJob.state));

  const c = $derived(run.counts ?? {});
  const added = $derived((c.added ?? 0) + (c.already ?? 0));
  const needInvite = $derived(d.people.filter((p) => p.outcome === 'privacy' || p.outcome === 'recently_left'));
  const inviteTargets = $derived(needInvite.filter((p) => p.inviteStatus !== 'sent'));
  const invitesSent = $derived(needInvite.filter((p) => p.inviteStatus === 'sent').length);
  const leftover = $derived(d.people.filter((p) => p.outcome === 'pending' || p.outcome === 'failed'));

  let message = $state('');
  let messageFor = '';
  $effect(() => {
    if (d.inviteMessage && messageFor !== run.id) {
      message = d.inviteMessage;
      messageFor = run.id;
    }
  });

  type Filter = 'all' | 'added' | 'invite' | 'none' | 'failed' | 'pending';
  let filter = $state<Filter>('all');
  const tabs = $derived(
    [
      { key: 'all', label: 'Everyone', n: d.people.length },
      { key: 'added', label: 'Added', n: added },
      { key: 'invite', label: 'Need an invite', n: c.invite ?? 0 },
      { key: 'none', label: 'Not on WhatsApp', n: c.notOnWhatsApp ?? 0 },
      { key: 'failed', label: "Couldn't add", n: c.failed ?? 0 },
      { key: 'pending', label: 'Not added yet', n: c.pending ?? 0 },
    ].filter((t) => t.key === 'all' || t.n > 0) as { key: Filter; label: string; n: number }[],
  );
  const shown = $derived(d.people.filter((p) => filter === 'all' || outcomeOf(p.outcome).group === filter));

  let confirmInvites = $state(false);
  let actionBusy = $state(false);
  let actionError = $state<string | null>(null);

  async function sendInvites() {
    actionBusy = true;
    actionError = null;
    try {
      const res = await api.post<{ job: Job }>(`/api/workflows/course-groups/runs/${run.id}/invites`, { message });
      app.setJob(res.job);
      confirmInvites = false;
    } catch (e) {
      actionError = (e as ApiError).message;
    } finally {
      actionBusy = false;
    }
  }

  async function addRemaining() {
    actionBusy = true;
    actionError = null;
    try {
      const res = await api.post<{ runId: string; job: Job }>('/api/workflows/course-groups/run', {
        mode: 'add',
        communityId: d.communityId,
        communityName: d.communityName,
        groupId: d.groupId,
        groupName: d.groupName,
        description: '',
        settings: d.settings,
        people: leftover.map((p) => ({ row: p.row, name: p.name, phone: p.phone, display: p.display })),
      });
      app.setJob(res.job);
      router.go(`/runs/${res.runId}`);
    } catch (e) {
      actionError = (e as ApiError).message;
    } finally {
      actionBusy = false;
    }
  }

  async function saveCsv() {
    actionError = null;
    try {
      await download(`/api/workflows/course-groups/runs/${run.id}/export`, `${d.groupName} results.csv`);
    } catch (e) {
      actionError = (e as ApiError).message;
    }
  }

  const inviteSeconds = $derived.by(() => {
    const p = app.info?.pacing;
    if (!p) return 0;
    return inviteTargets.length * (2 + (p.invitePauseMinSeconds + p.invitePauseMaxSeconds) / 2);
  });

  const heading = $derived.by(() => {
    switch (run.status) {
      case 'completed':
        return d.mode === 'create' ? `“${d.groupName}” is ready` : `People added to “${d.groupName}”`;
      case 'stopped':
        return `Stopped early: “${d.groupName}”`;
      case 'interrupted':
        return `“${d.groupName}” was interrupted`;
      case 'failed':
        return `“${d.groupName}” didn't finish`;
    }
    return d.groupName;
  });
</script>

{#if mainJob}
  <Progress job={mainJob} total={d.people.length} />
{:else if run.status === 'running'}
  <div class="loading"><Spinner size={24} /> <span>Loading the latest progress…</span></div>
{:else}
  <div class="result">
    <header class="head">
      <span class="hicon {run.status}">
        {#if run.status === 'completed'}<PartyPopper size={26} aria-hidden={true} />
        {:else if run.status === 'failed'}<CircleAlert size={26} aria-hidden={true} />
        {:else}<CircleStop size={26} aria-hidden={true} />{/if}
      </span>
      <div>
        <h1>{heading}</h1>
        <p class="muted">
          {d.mode === 'create' ? 'New group' : 'Added people'} in {d.communityName} · {when(run.startedAt)}
        </p>
      </div>
    </header>

    {#if run.error}
      <Banner tone={run.status === 'failed' ? 'danger' : 'warning'}>{run.error}</Banner>
    {/if}
    {#each d.warnings ?? [] as w (w)}
      <Banner tone="warning">{w}</Banner>
    {/each}

    <div class="tiles">
      <div class="tile success">
        <span class="tn tabular">{num(added)}</span>
        <span class="tl">Added</span>
        {#if c.already}<span class="ts">including {num(c.already)} already in the group</span>{/if}
      </div>
      <div class="tile warning">
        <span class="tn tabular">{num(c.invite ?? 0)}</span>
        <span class="tl">Couldn't add (privacy settings)</span>
        {#if invitesSent}<span class="ts">{num(invitesSent)} sent an invite</span>{/if}
      </div>
      <div class="tile neutral">
        <span class="tn tabular">{num(c.notOnWhatsApp ?? 0)}</span>
        <span class="tl">Not on WhatsApp</span>
      </div>
      <div class="tile danger">
        <span class="tn tabular">{num((c.failed ?? 0) + (c.pending ?? 0))}</span>
        <span class="tl">{c.pending ? 'Failed or not added yet' : 'Failed'}</span>
      </div>
    </div>

    {#if leftover.length && d.groupId}
      <Banner tone="info" title="{people(leftover.length)} still to add">
        You can try adding them again now. The ones already added aren't touched.
        {#snippet actions()}
          <Button variant="primary" size="sm" icon={UserPlus} loading={actionBusy} disabled={!!app.activeJob || !app.connected} onclick={addRemaining}>
            Add the remaining {people(leftover.length)}
          </Button>
        {/snippet}
      </Banner>
    {/if}

    {#if d.inviteLink}
      <section class="card invite" aria-labelledby="invite-h">
        <div class="invite-head">
          <span class="iicon"><Link2 size={20} aria-hidden={true} /></span>
          <div>
            <h2 id="invite-h">
              {needInvite.length ? `Invite the ${people(needInvite.length)} who couldn't be added` : 'Group invite link'}
            </h2>
            <p class="muted">
              {#if needInvite.length}
                Their WhatsApp privacy settings only let people they know add them to groups, so they need to join with
                the group's invite link instead.
              {:else}
                Share this link with anyone else who should join the group.
              {/if}
            </p>
          </div>
        </div>

        <div class="link-row">
          <input class="link" readonly value={d.inviteLink} aria-label="Group invite link" onfocus={(e) => (e.currentTarget as HTMLInputElement).select()} />
          <Button variant="primary" icon={Copy} onclick={() => copyText(d.inviteLink!, 'Invite link copied')}>Copy invite link</Button>
        </div>

        {#if needInvite.length}
          <div class="message">
            <label for="invite-msg">A message you can paste into WhatsApp or an email</label>
            <textarea id="invite-msg" rows="4" bind:value={message}></textarea>
            <div class="message-actions">
              <Button icon={Copy} onclick={() => copyText(message, 'Message copied')}>Copy message</Button>
            </div>
          </div>

          <div class="send">
            {#if invitesActive && inviteJob}
              <div class="send-progress" role="status">
                <p class="send-title">
                  {#if inviteJob.state === 'waiting'}{inviteJob.message}{:else}Sending invites one at a time… {num(inviteJob.done)} of {num(inviteJob.total)}{/if}
                </p>
                <ProgressBar value={inviteJob.total ? inviteJob.done / inviteJob.total : 0} label="Sending invites" />
                <div class="send-controls">
                  {#if inviteJob.state === 'paused'}
                    <Button size="sm" onclick={() => api.post(`/api/jobs/${inviteJob.id}/resume`).then((j) => app.setJob(j as Job))}>Continue</Button>
                  {:else}
                    <Button size="sm" onclick={() => api.post(`/api/jobs/${inviteJob.id}/pause`).then((j) => app.setJob(j as Job))}>Pause</Button>
                  {/if}
                  <Button size="sm" variant="danger" onclick={() => api.post(`/api/jobs/${inviteJob.id}/stop`).then((j) => app.setJob(j as Job))}>Stop sending</Button>
                </div>
              </div>
            {:else if inviteTargets.length}
              <div class="send-text">
                <p class="send-title">Or let the app send each of them an invite in WhatsApp</p>
                <p class="muted small">
                  This sends a personal WhatsApp message from {app.status.account?.name || 'the connected account'} to each of
                  the {people(inviteTargets.length)}, one at a time. Only use it if they're expecting to hear from you.
                </p>
              </div>
              <Button icon={Send} disabled={!!app.activeJob || !app.connected} onclick={() => (confirmInvites = true)}>
                Send them an invite in WhatsApp
              </Button>
            {:else}
              <p class="sent-all"><Send size={16} aria-hidden={true} /> Everyone who needed an invite has been sent one.</p>
            {/if}
          </div>
        {/if}
      </section>
    {/if}

    {#if actionError}<Banner tone="danger" live>{actionError}</Banner>{/if}

    <section class="card people" aria-labelledby="people-h">
      <div class="people-head">
        <h2 id="people-h">Everyone on the list</h2>
        <div class="tabs" role="group" aria-label="Show">
          {#each tabs as t (t.key)}
            <button class="tab" class:active={filter === t.key} aria-pressed={filter === t.key} onclick={() => (filter = t.key)}>
              {t.label} <span class="tabular">{num(t.n)}</span>
            </button>
          {/each}
        </div>
      </div>
      <ul class="plist">
        {#each shown as p (p.row)}
          {@const o = outcomeOf(p.outcome)}
          <li>
            <span class="pname">
              <span class="pn">{p.name || p.display}</span>
              {#if p.name}<span class="pp tabular">{p.display}</span>{/if}
            </span>
            <span class="pout">
              <Pill tone={o.tone} icon={o.icon} text={o.text} />
              {#if p.inviteStatus === 'sent'}<Pill tone="success" icon={Send} text="Invite sent" />{/if}
              {#if p.inviteStatus === 'failed'}<Pill tone="danger" text="Invite not sent" />{/if}
            </span>
          </li>
        {/each}
      </ul>
    </section>

    <div class="actions">
      <Button icon={Download} onclick={saveCsv}>Save results as a spreadsheet</Button>
      {#if d.inviteLink}
        <Button icon={Copy} onclick={() => copyText(d.inviteLink!, 'Invite link copied')}>Copy invite link</Button>
      {/if}
      <span class="spacer"></span>
      <Button variant="primary" icon={House} onclick={() => router.go('/')}>Back to home</Button>
    </div>
  </div>
{/if}

<Modal open={confirmInvites} title="Send {people(inviteTargets.length)} an invite?" onclose={() => (confirmInvites = false)} width={540}>
  <p>
    Each of them will get a personal WhatsApp message from <strong>{app.status.account?.name || 'the connected account'}</strong>
    ({app.status.account?.phone}) with your message and the group link.
  </p>
  <p>
    To protect the account, messages go out one at a time with a pause between them. This takes {duration(inviteSeconds)}.
    You can stop at any time.
  </p>
  {#if actionError}<p class="err">{actionError}</p>{/if}
  {#snippet actions()}
    <Button onclick={() => (confirmInvites = false)}>Cancel</Button>
    <Button variant="primary" icon={Send} loading={actionBusy} onclick={sendInvites}>Send {num(inviteTargets.length)} invites</Button>
  {/snippet}
</Modal>

<style>
  .loading {
    display: flex;
    gap: 0.75rem;
    align-items: center;
    justify-content: center;
    padding: 4rem;
    color: var(--text-secondary);
  }
  .result {
    max-width: 920px;
    margin: 0 auto;
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .head {
    display: flex;
    gap: 1rem;
    align-items: center;
  }
  .head h1 {
    font-size: 1.75rem;
  }
  .hicon {
    flex: none;
    width: 56px;
    height: 56px;
    border-radius: 50%;
    display: grid;
    place-items: center;
    background: var(--warning-soft);
    color: var(--warning);
  }
  .hicon.completed {
    background: var(--success-soft);
    color: var(--success);
  }
  .hicon.failed {
    background: var(--danger-soft);
    color: var(--danger);
  }
  .tiles {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 0.9rem;
  }
  .tile {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-top: 4px solid;
    border-radius: var(--radius);
    padding: 1.1rem 1.15rem;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    box-shadow: var(--shadow-sm);
  }
  .tile.success {
    border-top-color: var(--success);
  }
  .tile.warning {
    border-top-color: var(--warning);
  }
  .tile.neutral {
    border-top-color: var(--border-strong);
  }
  .tile.danger {
    border-top-color: var(--danger);
  }
  .tn {
    font-size: 2.25rem;
    font-weight: 700;
    line-height: 1.1;
  }
  .tl {
    font-weight: 600;
    line-height: 1.35;
  }
  .ts {
    font-size: 0.8125rem;
    color: var(--text-secondary);
  }
  .card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 1.6rem;
    box-shadow: var(--shadow-sm);
  }
  .invite {
    display: flex;
    flex-direction: column;
    gap: 1.1rem;
  }
  .invite-head {
    display: flex;
    gap: 0.9rem;
    align-items: flex-start;
  }
  .invite-head h2 {
    font-size: 1.1875rem;
  }
  .invite-head p {
    margin-top: 0.2rem;
  }
  .iicon {
    flex: none;
    width: 40px;
    height: 40px;
    border-radius: 12px;
    display: grid;
    place-items: center;
    background: var(--warning-soft);
    color: var(--warning-text);
  }
  .link-row {
    display: flex;
    gap: 0.6rem;
  }
  .link {
    flex: 1;
    min-width: 0;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--bg-raised);
    padding: 0.6rem 0.85rem;
    font-family: var(--font-mono);
    font-size: 0.9375rem;
  }
  .message {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .message label {
    font-weight: 600;
    font-size: 0.9375rem;
  }
  textarea {
    width: 100%;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--bg-raised);
    padding: 0.7rem 0.85rem;
    line-height: 1.5;
    resize: vertical;
  }
  textarea:focus-visible,
  .link:focus-visible {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 3px rgba(131, 17, 79, 0.18);
  }
  .message-actions {
    display: flex;
    justify-content: flex-end;
  }
  .send {
    display: flex;
    gap: 1rem;
    align-items: center;
    justify-content: space-between;
    border-top: 1px solid var(--border-subtle);
    padding-top: 1.1rem;
  }
  .send-text {
    max-width: 520px;
  }
  .send-title {
    font-weight: 600;
  }
  .send-progress {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
  }
  .send-controls {
    display: flex;
    gap: 0.5rem;
  }
  .sent-all {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    color: var(--success-text);
    font-weight: 560;
  }
  .people-head {
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
    margin-bottom: 0.6rem;
  }
  .people-head h2 {
    font-size: 1.1875rem;
  }
  .tabs {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }
  .tab {
    border: 1px solid var(--border);
    background: var(--bg-raised);
    border-radius: 999px;
    padding: 0.35rem 0.8rem;
    font-size: 0.875rem;
    font-weight: 560;
    cursor: pointer;
  }
  .tab span {
    color: var(--text-secondary);
    margin-left: 0.2rem;
  }
  .tab:hover {
    border-color: var(--border-strong);
  }
  .tab.active {
    border-color: var(--accent);
    background: var(--bg-accent-soft);
    color: var(--accent);
  }
  .plist {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .plist li {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    padding: 0.7rem 0.25rem;
  }
  .plist li + li {
    border-top: 1px solid var(--border-subtle);
  }
  .pname {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .pn {
    font-weight: 580;
  }
  .pp {
    font-size: 0.875rem;
    color: var(--text-secondary);
  }
  .pout {
    display: flex;
    gap: 0.4rem;
    flex-wrap: wrap;
    justify-content: flex-end;
  }
  .actions {
    display: flex;
    gap: 0.6rem;
    flex-wrap: wrap;
    align-items: center;
  }
  .spacer {
    flex: 1;
  }
  .err {
    color: var(--danger);
  }
  @media (max-width: 860px) {
    .tiles {
      grid-template-columns: 1fr 1fr;
    }
    .send {
      flex-direction: column;
      align-items: stretch;
    }
    .link-row {
      flex-direction: column;
    }
  }
</style>
