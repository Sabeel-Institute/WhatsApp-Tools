<script lang="ts">
  import Pause from '@lucide/svelte/icons/pause';
  import Play from '@lucide/svelte/icons/play';
  import Square from '@lucide/svelte/icons/square';
  import Coffee from '@lucide/svelte/icons/coffee';
  import Button from '../../components/Button.svelte';
  import Banner from '../../components/Banner.svelte';
  import Modal from '../../components/Modal.svelte';
  import ProgressBar from '../../components/ProgressBar.svelte';
  import Spinner from '../../components/Spinner.svelte';
  import { api, ApiError } from '../../lib/api';
  import { app } from '../../lib/state.svelte';
  import { duration, num } from '../../lib/format';
  import type { Job } from '../../lib/types';
  import { outcomeOf } from './outcomes';

  let { job, total }: { job: Job; total: number } = $props();
  let confirmStop = $state(false);
  let busy = $state(false);
  let error = $state<string | null>(null);

  async function act(action: 'pause' | 'resume' | 'stop') {
    busy = true;
    error = null;
    try {
      app.setJob(await api.post<Job>(`/api/jobs/${job.id}/${action}`));
      confirmStop = false;
    } catch (e) {
      error = (e as ApiError).message;
    } finally {
      busy = false;
    }
  }

  const added = $derived((job.counts.added ?? 0) + (job.counts.already ?? 0));
  const fraction = $derived(job.total ? job.done / job.total : 0);
  const stateLabel = $derived(
    { running: 'Working', paused: 'Paused', waiting: 'Waiting', stopping: 'Stopping' }[job.state as string] ??
      'Working',
  );
  const others = $derived(
    [
      job.counts.invite ? `${num(job.counts.invite)} need an invite` : '',
      job.counts.notOnWhatsApp ? `${num(job.counts.notOnWhatsApp)} not on WhatsApp` : '',
      job.counts.failed ? `${num(job.counts.failed)} couldn't be added` : '',
    ].filter(Boolean),
  );
</script>

<div class="wrap">
  <section class="card main" aria-labelledby="job-title">
    <p class="label {job.state}">
      {#if job.state === 'running' || job.state === 'stopping'}<Spinner size={14} />{/if}
      {stateLabel}
    </p>
    <h1 id="job-title">{job.title}</h1>
    <p class="phase" aria-live="polite">
      {job.state === 'stopping'
        ? 'Finishing the current step, then stopping…'
        : job.state === 'paused'
          ? 'Waiting for you to continue'
          : job.phase}
    </p>

    <div class="numbers">
      <p class="big tabular" aria-live="polite">
        Added <strong>{num(added)}</strong> of {num(total)}
      </p>
      {#if job.etaSeconds > 0 && job.state === 'running'}
        <p class="eta">{duration(job.etaSeconds)} left</p>
      {/if}
    </div>
    <ProgressBar value={fraction} label="Progress" size="lg" tone={job.state === 'paused' ? 'muted' : 'accent'} />
    <p class="sub tabular">
      {num(job.done)} of {num(job.total)} people done{#if others.length}&nbsp;· {others.join(' · ')}{/if}
    </p>

    {#if job.state === 'waiting'}
      <Banner tone="warning" live>{job.message}</Banner>
    {:else if job.state === 'paused'}
      <Banner tone="info" title="Paused" live>
        Nothing more is being added. Everyone added so far stays in the group. Choose Continue when you're ready.
      </Banner>
    {/if}
    {#if error}<Banner tone="danger" live>{error}</Banner>{/if}

    <div class="controls">
      {#if job.state === 'paused'}
        <Button variant="primary" icon={Play} loading={busy} onclick={() => act('resume')}>Continue</Button>
      {:else}
        <Button icon={Pause} loading={busy} disabled={job.state === 'stopping'} onclick={() => act('pause')}
          >Pause</Button
        >
      {/if}
      <Button variant="danger" icon={Square} disabled={job.state === 'stopping'} onclick={() => (confirmStop = true)}>
        Stop
      </Button>
    </div>
  </section>

  <section class="card feed" aria-labelledby="feed-title">
    <h2 id="feed-title">Latest results</h2>
    {#if job.recent.length === 0}
      <p class="muted empty">Results will appear here as people are added.</p>
    {:else}
      <ul>
        {#each job.recent.slice(0, 12) as e (e.at + e.title + e.outcome)}
          {@const o = outcomeOf(e.outcome)}
          {@const Icon = o.icon}
          <li>
            <span class="ficon {o.tone}"><Icon size={16} aria-hidden={true} /></span>
            <span class="ftext">
              <span class="fname">{e.title}</span>
              {#if e.detail}<span class="fdetail tabular">{e.detail}</span>{/if}
            </span>
            <span class="foutcome {o.tone}">{o.text}</span>
          </li>
        {/each}
      </ul>
    {/if}
  </section>

  <p class="keep-open">
    <Coffee size={17} aria-hidden={true} /> You can use other programs meanwhile. Keep this app open until it finishes.
  </p>
</div>

<Modal open={confirmStop} title="Stop adding people?" onclose={() => (confirmStop = false)}>
  <p>Everyone added so far stays in the group. The app finishes the step it's on, then stops.</p>
  <p>You'll be able to add the rest later from the results.</p>
  {#snippet actions()}
    <Button onclick={() => (confirmStop = false)}>Keep going</Button>
    <Button variant="danger" icon={Square} loading={busy} onclick={() => act('stop')}>Stop</Button>
  {/snippet}
</Modal>

<style>
  .wrap {
    max-width: 820px;
    margin: 0 auto;
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 2rem;
    box-shadow: var(--shadow);
  }
  .main {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
  .label {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    font-size: 0.8125rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.07em;
    color: var(--text-accent);
  }
  .label.paused {
    color: var(--info);
  }
  .label.waiting {
    color: var(--warning-text);
  }
  h1 {
    font-size: 1.625rem;
    margin-top: -0.4rem;
  }
  .phase {
    color: var(--text-secondary);
    margin-top: -0.4rem;
  }
  .numbers {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    gap: 1rem;
    margin-top: 0.6rem;
  }
  .big {
    font-size: 1.375rem;
  }
  .big strong {
    font-size: 2.25rem;
    font-weight: 700;
    color: var(--text-primary);
  }
  .eta {
    color: var(--text-secondary);
    font-weight: 560;
  }
  .sub {
    color: var(--text-secondary);
    font-size: 0.9375rem;
  }
  .controls {
    display: flex;
    gap: 0.6rem;
    margin-top: 0.4rem;
  }
  .feed h2 {
    font-size: 1.0625rem;
    margin-bottom: 0.75rem;
  }
  .empty {
    padding: 0.5rem 0;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  li {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.6rem 0;
    animation: fade-in 0.3s ease-out;
  }
  li + li {
    border-top: 1px solid var(--border-subtle);
  }
  @keyframes fade-in {
    from {
      opacity: 0;
      transform: translateY(-4px);
    }
  }
  .ficon {
    flex: none;
    width: 30px;
    height: 30px;
    border-radius: 50%;
    display: grid;
    place-items: center;
  }
  .ficon.success {
    background: var(--success-soft);
    color: var(--success);
  }
  .ficon.warning {
    background: var(--warning-soft);
    color: var(--warning);
  }
  .ficon.danger {
    background: var(--danger-soft);
    color: var(--danger);
  }
  .ficon.neutral {
    background: var(--neutral-soft);
    color: var(--text-secondary);
  }
  .ficon.accent {
    background: var(--bg-accent-soft);
    color: var(--accent);
  }
  .ftext {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .fname {
    font-weight: 580;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .fdetail {
    font-size: 0.8125rem;
    color: var(--text-secondary);
  }
  .foutcome {
    font-size: 0.875rem;
    font-weight: 600;
    white-space: nowrap;
  }
  .foutcome.success {
    color: var(--success-text);
  }
  .foutcome.warning {
    color: var(--warning-text);
  }
  .foutcome.danger {
    color: var(--danger);
  }
  .foutcome.neutral {
    color: var(--text-secondary);
  }
  .foutcome.accent {
    color: var(--accent);
  }
  .keep-open {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    color: var(--text-secondary);
    font-size: 0.9375rem;
  }
</style>
