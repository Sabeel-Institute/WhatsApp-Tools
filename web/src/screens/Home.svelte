<script lang="ts">
  import { onMount } from 'svelte';
  import ChevronRight from '@lucide/svelte/icons/chevron-right';
  import ArrowRight from '@lucide/svelte/icons/arrow-right';
  import History from '@lucide/svelte/icons/history';
  import CircleCheck from '@lucide/svelte/icons/circle-check';
  import CircleStop from '@lucide/svelte/icons/circle-stop';
  import CircleAlert from '@lucide/svelte/icons/circle-alert';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import Button from '../components/Button.svelte';
  import Banner from '../components/Banner.svelte';
  import ProgressBar from '../components/ProgressBar.svelte';
  import Spinner from '../components/Spinner.svelte';
  import { api, ApiError } from '../lib/api';
  import { app, router } from '../lib/state.svelte';
  import { when, num } from '../lib/format';
  import { actionIcon } from '../workflows/registry';
  import type { Run } from '../lib/types';

  let runs = $state<Run[] | null>(null);
  let loadError = $state<string | null>(null);

  async function load() {
    try {
      runs = await api.get<Run[]>('/api/runs');
      loadError = null;
    } catch (e) {
      loadError = (e as ApiError).message;
    }
  }
  onMount(load);

  // Refresh the list when a job finishes.
  let lastActive: string | null = null;
  $effect(() => {
    const id = app.activeJobId;
    if (lastActive && !id) load();
    lastActive = id;
  });

  const job = $derived(app.activeJob);
  const showJob = $derived(job && job.runId);
  const workflows = $derived(app.info?.workflows ?? []);
  const reconnecting = $derived(app.status.state === 'connecting');

  const statusInfo: Record<string, { text: string; tone: string }> = {
    completed: { text: 'Finished', tone: 'success' },
    stopped: { text: 'Stopped early', tone: 'warning' },
    failed: { text: "Didn't finish", tone: 'danger' },
    interrupted: { text: 'Interrupted', tone: 'warning' },
    running: { text: 'In progress', tone: 'info' },
  };
</script>

<div class="home">
  <div class="greeting">
    <p class="eyebrow">Assalamu alaikum</p>
    <h1>What would you like to do?</h1>
  </div>

  {#if reconnecting}
    <div class="block">
      <Banner tone="info" title="Reconnecting to WhatsApp…" live>
        The app reconnects by itself when the internet comes back. You can start preparing a list in the meantime.
      </Banner>
    </div>
  {/if}

  {#if showJob && job}
    <section class="job" aria-label="In progress">
      <div class="job-text">
        <p class="job-label">
          {#if job.state === 'paused'}Paused{:else if job.state === 'waiting'}Waiting{:else}In progress{/if}
        </p>
        <h2>{job.title}</h2>
        <p class="muted small">{job.message || job.phase}</p>
        <div class="job-bar">
          <ProgressBar value={job.total ? job.done / job.total : 0} label="Progress" size="sm" />
          <span class="small tabular">{num(job.done)} of {num(job.total)}</span>
        </div>
      </div>
      <Button variant="primary" iconRight={ArrowRight} onclick={() => router.go(`/runs/${job.runId}`)}>
        View progress
      </Button>
    </section>
  {/if}

  {#each workflows as wf (wf.id)}
    <section class="workflow" aria-labelledby="wf-{wf.id}">
      <div class="section-head">
        <h2 id="wf-{wf.id}">{wf.title}</h2>
        <p class="muted">{wf.description}</p>
      </div>
      <div class="actions">
        {#each wf.actions as action (action.id)}
          {@const Icon = actionIcon(action.icon)}
          <button
            class="action"
            class:primary={action.primary}
            onclick={() => router.go(`/w/${wf.id}/${action.id}`)}
          >
            <span class="action-icon"><Icon size={26} aria-hidden={true} /></span>
            <span class="action-text">
              <span class="action-label">{action.label}</span>
              <span class="action-desc">{action.description}</span>
            </span>
            <span class="action-go"><ArrowRight size={20} aria-hidden={true} /></span>
          </button>
        {/each}
      </div>
    </section>
  {/each}

  <section class="recent" aria-labelledby="recent-h">
    <div class="section-head">
      <h2 id="recent-h">Recent activity</h2>
    </div>
    {#if loadError}
      <Banner tone="danger">{loadError}</Banner>
    {:else if runs === null}
      <div class="loading-list"><Spinner size={20} /> <span class="muted">Loading…</span></div>
    {:else if runs.length === 0}
      <div class="empty">
        <span class="empty-icon"><History size={26} aria-hidden={true} /></span>
        <p class="empty-title">Nothing here yet</p>
        <p class="muted">When you create a group or add people, it will appear here so you can look at the results again.</p>
      </div>
    {:else}
      <ul class="run-list">
        {#each runs as run (run.id)}
          {@const s = statusInfo[run.status] ?? statusInfo.completed}
          <li>
            <a href="#/runs/{run.id}" class="run">
              <span class="run-icon {s.tone}">
                {#if run.status === 'completed'}<CircleCheck size={20} aria-hidden={true} />
                {:else if run.status === 'running'}<LoaderCircle size={20} aria-hidden={true} />
                {:else if run.status === 'failed'}<CircleAlert size={20} aria-hidden={true} />
                {:else}<CircleStop size={20} aria-hidden={true} />{/if}
              </span>
              <span class="run-main">
                <span class="run-title">{run.title}</span>
                <span class="run-sub small">
                  {run.kind === 'add' ? 'Added people' : 'New group'}{run.subtitle ? ` · ${run.subtitle}` : ''} · {when(run.startedAt)}
                </span>
              </span>
              <span class="run-result">
                <span class="run-headline">{run.headline}</span>
                {#if run.status !== 'completed'}<span class="run-status {s.tone}">{s.text}</span>{/if}
              </span>
              <ChevronRight size={20} aria-hidden={true} class="chev" />
            </a>
          </li>
        {/each}
      </ul>
    {/if}
  </section>
</div>

<style>
  .home {
    max-width: 920px;
    margin: 0 auto;
    display: flex;
    flex-direction: column;
    gap: 2.25rem;
  }
  .greeting h1 {
    font-size: 2rem;
  }
  .eyebrow {
    color: var(--warning-text);
    font-weight: 600;
    letter-spacing: 0.02em;
    margin-bottom: 0.2rem;
  }
  .block {
    margin-top: -1rem;
  }
  .section-head {
    margin-bottom: 1rem;
  }
  .section-head h2 {
    font-size: 1.25rem;
  }
  .section-head p {
    margin-top: 0.15rem;
  }
  .actions {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
  }
  .action {
    display: flex;
    align-items: center;
    gap: 1rem;
    text-align: left;
    padding: 1.35rem 1.25rem;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    cursor: pointer;
    box-shadow: var(--shadow-sm);
    transition:
      border-color 0.15s ease,
      box-shadow 0.15s ease,
      transform 0.15s ease;
  }
  .action:hover {
    border-color: var(--border-strong);
    box-shadow: var(--shadow);
    transform: translateY(-1px);
  }
  .action:focus-visible {
    outline: none;
    box-shadow: var(--focus);
  }
  .action-icon {
    flex: none;
    width: 52px;
    height: 52px;
    border-radius: 14px;
    display: grid;
    place-items: center;
    background: var(--bg-inset);
    color: var(--text-accent);
  }
  .primary .action-icon {
    background: var(--accent);
    color: var(--on-accent);
  }
  .primary {
    border-color: #dcc2c9;
  }
  .action-text {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    flex: 1;
    min-width: 0;
  }
  .action-label {
    font-weight: 650;
    font-size: 1.0625rem;
  }
  .action-desc {
    color: var(--text-secondary);
    font-size: 0.9375rem;
    line-height: 1.45;
  }
  .action-go {
    color: var(--text-secondary);
    flex: none;
  }
  .action:hover .action-go {
    color: var(--accent);
  }
  .job {
    display: flex;
    align-items: center;
    gap: 1.5rem;
    padding: 1.25rem 1.5rem;
    background: var(--bg-surface);
    border: 1px solid #dcc2c9;
    border-left: 4px solid var(--accent);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow);
  }
  .job-text {
    flex: 1;
    min-width: 0;
  }
  .job-label {
    font-size: 0.8125rem;
    font-weight: 650;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-accent);
  }
  .job h2 {
    font-size: 1.125rem;
    margin-top: 0.1rem;
  }
  .job-bar {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-top: 0.6rem;
    max-width: 420px;
  }
  .job-bar span {
    white-space: nowrap;
    color: var(--text-secondary);
  }
  .run-list {
    list-style: none;
    margin: 0;
    padding: 0;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    overflow: hidden;
  }
  .run-list li + li {
    border-top: 1px solid var(--border-subtle);
  }
  .run {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 1rem 1.25rem;
    color: inherit;
    text-decoration: none;
  }
  .run:hover {
    background: var(--bg-hover);
  }
  .run:focus-visible {
    outline-offset: -3px;
  }
  .run-icon {
    flex: none;
    width: 38px;
    height: 38px;
    border-radius: 50%;
    display: grid;
    place-items: center;
  }
  .run-icon.success {
    background: var(--success-soft);
    color: var(--success);
  }
  .run-icon.warning {
    background: var(--warning-soft);
    color: var(--warning);
  }
  .run-icon.danger {
    background: var(--danger-soft);
    color: var(--danger);
  }
  .run-icon.info {
    background: var(--info-soft);
    color: var(--info);
  }
  .run-main {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .run-title {
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .run-sub {
    color: var(--text-secondary);
  }
  .run-result {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    text-align: right;
  }
  .run-headline {
    font-weight: 560;
    font-size: 0.9375rem;
  }
  .run-status {
    font-size: 0.8125rem;
    font-weight: 600;
  }
  .run-status.warning {
    color: var(--warning-text);
  }
  .run-status.danger {
    color: var(--danger);
  }
  .run-status.info {
    color: var(--info);
  }
  .run :global(.chev) {
    color: var(--text-muted);
    flex: none;
  }
  .empty {
    text-align: center;
    padding: 2.5rem 1.5rem;
    background: var(--bg-surface);
    border: 1px dashed var(--border-strong);
    border-radius: var(--radius-lg);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.4rem;
  }
  .empty .muted {
    max-width: 420px;
  }
  .empty-icon {
    width: 52px;
    height: 52px;
    border-radius: 50%;
    display: grid;
    place-items: center;
    background: var(--bg-inset);
    color: var(--text-secondary);
    margin-bottom: 0.4rem;
  }
  .empty-title {
    font-weight: 620;
    font-size: 1.0625rem;
  }
  .loading-list {
    display: flex;
    gap: 0.6rem;
    align-items: center;
    padding: 1rem;
  }
  @media (max-width: 760px) {
    .actions {
      grid-template-columns: 1fr;
    }
    .job {
      flex-direction: column;
      align-items: stretch;
    }
    .run-result {
      display: none;
    }
  }
</style>
