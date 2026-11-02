<script lang="ts">
  import ArrowLeft from '@lucide/svelte/icons/arrow-left';
  import ArrowRight from '@lucide/svelte/icons/arrow-right';
  import House from '@lucide/svelte/icons/house';
  import RotateCcw from '@lucide/svelte/icons/rotate-ccw';
  import Play from '@lucide/svelte/icons/play';
  import Button from '../../components/Button.svelte';
  import Banner from '../../components/Banner.svelte';
  import Modal from '../../components/Modal.svelte';
  import Stepper from '../../components/Stepper.svelte';
  import ImportStep from '../../people/ImportStep.svelte';
  import MatchStep from '../../people/MatchStep.svelte';
  import ReviewStep from '../../people/ReviewStep.svelte';
  import DetailsStep from './DetailsStep.svelte';
  import ConfirmStep from './ConfirmStep.svelte';
  import { app, router } from '../../lib/state.svelte';
  import { people } from '../../lib/format';
  import { draftFor, resetDraft, type Mode } from './draft.svelte';

  let { action }: { action: string } = $props();

  const mode = $derived<Mode | null>(action === 'create' || action === 'add' ? action : null);
  // Bumped by "Start over" so the derived draft is fetched afresh.
  let version = $state(0);
  const draft = $derived.by(() => {
    void version;
    return mode ? draftFor(mode) : null;
  });
  let confirmReset = $state(false);

  $effect(() => {
    if (!mode) router.go('/');
  });

  const steps = $derived(
    mode === 'add' ? ['Import', 'Match', 'Review', 'Group', 'Add'] : ['Import', 'Match', 'Review', 'Details', 'Create'],
  );
  const title = $derived(mode === 'add' ? 'Add people to an existing group' : 'Create a course group');
  const busyElsewhere = $derived(!!app.activeJob && app.activeJob.kind !== 'check');

  const canContinue = $derived.by(() => {
    if (!draft) return false;
    const l = draft.list;
    switch (draft.step) {
      case 0:
        return !!l.table && !l.parsing;
      case 1:
        return l.mappingComplete;
      case 2:
        return !!l.review && !l.reviewing && l.selected.length > 0;
      case 3:
        return draft.detailsComplete;
    }
    return false;
  });

  const blockedReason = $derived.by(() => {
    if (!draft) return '';
    const l = draft.list;
    if (draft.step === 1 && l.mapping.phone < 0) return 'Choose the phone number column to continue.';
    if (draft.step === 1 && !l.mappingComplete) return 'Choose the name column(s), or pick “Don’t use names”.';
    if (draft.step === 2 && l.review && l.selected.length === 0) return 'Tick at least one person to continue.';
    if (draft.step === 3 && draft.mode === 'create' && !draft.groupName.trim() && draft.community)
      return 'Give the group a name to continue.';
    return '';
  });

  async function next() {
    if (!draft || !canContinue) return;
    if (draft.step === 1) draft.list.rememberChoices();
    draft.go(draft.step + 1);
  }

  async function start() {
    if (!draft || !mode) return;
    const runId = await draft.start();
    if (runId) {
      resetDraft(mode);
      router.go(`/runs/${runId}`);
    }
  }

  function startOver() {
    if (!mode) return;
    resetDraft(mode);
    version++;
    confirmReset = false;
  }
</script>

{#if draft}
  <div class="wizard">
    <div class="top">
      <div class="title-row">
        <a class="home-link" href="#/"><House size={16} aria-hidden={true} /> Home</a>
        {#if draft.list.table}
          <Button variant="quiet" size="sm" icon={RotateCcw} onclick={() => (confirmReset = true)}>Start over</Button>
        {/if}
      </div>
      <h1>{title}</h1>
      <Stepper {steps} current={draft.step} reachable={draft.reached} onstep={(i) => draft?.go(i)} />
    </div>

    <section class="card" aria-live="off">
      {#if draft.step === 0}
        <ImportStep list={draft.list} />
      {:else if draft.step === 1}
        <MatchStep list={draft.list} />
      {:else if draft.step === 2}
        <ReviewStep list={draft.list} />
      {:else if draft.step === 3}
        <DetailsStep {draft} />
      {:else}
        <ConfirmStep {draft} />
      {/if}
    </section>

    {#if draft.step === 4 && busyElsewhere}
      <div class="busy">
        <Banner tone="info" title="Another task is still running">
          “{app.activeJob?.title}” needs to finish before this one can start.
          <a href="#/runs/{app.activeJob?.runId}">See its progress</a>.
        </Banner>
      </div>
    {/if}

    <div class="footer">
      <div class="left">
        {#if draft.step > 0}
          <Button icon={ArrowLeft} onclick={() => draft?.go(draft.step - 1)}>Back</Button>
        {:else}
          <Button icon={ArrowLeft} onclick={() => router.go('/')}>Cancel</Button>
        {/if}
      </div>
      <div class="right">
        {#if draft.step === 2 && draft.list.review}
          <span class="count" aria-live="polite">{people(draft.list.selected.length)} will be added</span>
        {/if}
        {#if blockedReason}<span class="why">{blockedReason}</span>{/if}
        {#if draft.step < 4}
          <Button variant="primary" size="lg" iconRight={ArrowRight} disabled={!canContinue} onclick={next}>Continue</Button>
        {:else}
          <Button
            variant="primary"
            size="lg"
            icon={Play}
            loading={draft.starting}
            disabled={busyElsewhere || !app.connected}
            onclick={start}
          >
            {draft.mode === 'create'
              ? `Create group and add ${people(draft.list.selected.length)}`
              : `Add ${people(draft.list.selected.length)}`}
          </Button>
        {/if}
      </div>
    </div>
    {#if draft.step === 4 && !app.connected}
      <p class="offline">WhatsApp is reconnecting. The button becomes available as soon as it's back.</p>
    {/if}
  </div>

  <Modal open={confirmReset} title="Start over?" onclose={() => (confirmReset = false)}>
    <p>This clears the file and everything you've chosen so far.</p>
    {#snippet actions()}
      <Button onclick={() => (confirmReset = false)}>Keep my work</Button>
      <Button variant="danger" onclick={startOver}>Start over</Button>
    {/snippet}
  </Modal>
{/if}

<style>
  .wizard {
    max-width: 1000px;
    margin: 0 auto;
  }
  .top {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
    margin-bottom: 1.5rem;
  }
  .title-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    min-height: 34px;
  }
  .home-link {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    color: var(--text-secondary);
    text-decoration: none;
    font-weight: 560;
    font-size: 0.9375rem;
    padding: 0.25rem 0.5rem 0.25rem 0.25rem;
    border-radius: 6px;
  }
  .home-link:hover {
    color: var(--text-primary);
    background: var(--bg-hover);
  }
  h1 {
    font-size: 1.75rem;
  }
  .card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 2rem;
    box-shadow: var(--shadow);
  }
  .busy {
    margin-top: 1rem;
  }
  .footer {
    position: sticky;
    bottom: 0;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    margin-top: 1.25rem;
    padding: 1rem 0;
    background: linear-gradient(to bottom, rgba(246, 235, 221, 0), var(--bg-canvas) 30%);
  }
  .right {
    display: flex;
    align-items: center;
    gap: 1rem;
  }
  .count {
    font-weight: 600;
    color: var(--text-secondary);
  }
  .why {
    color: var(--text-secondary);
    font-size: 0.9375rem;
    max-width: 320px;
    text-align: right;
  }
  .offline {
    text-align: right;
    color: var(--warning-text);
    font-size: 0.9375rem;
  }
  @media (max-width: 640px) {
    .card {
      padding: 1.25rem;
    }
    .why,
    .count {
      display: none;
    }
  }
</style>
