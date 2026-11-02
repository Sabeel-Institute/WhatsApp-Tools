<script lang="ts">
  import { onMount } from 'svelte';
  import ArrowLeft from '@lucide/svelte/icons/arrow-left';
  import Banner from '../components/Banner.svelte';
  import Button from '../components/Button.svelte';
  import Spinner from '../components/Spinner.svelte';
  import { api, ApiError } from '../lib/api';
  import { app, router } from '../lib/state.svelte';
  import { resultScreen } from '../workflows/registry';
  import type { Run } from '../lib/types';

  let { id }: { id: string } = $props();
  let run = $state<Run | null>(null);
  let error = $state<string | null>(null);

  async function load() {
    try {
      run = await api.get<Run>(`/api/runs/${encodeURIComponent(id)}`);
      error = null;
    } catch (e) {
      error = (e as ApiError).message;
    }
  }
  onMount(load);

  // Reload the saved run whenever a job working on it reports progress, so
  // the result view always reflects what is on disk.
  let timer: ReturnType<typeof setTimeout> | undefined;
  $effect(() => {
    const jobs = Object.values(app.jobs).filter((j) => j.runId === id);
    const stamp = jobs.map((j) => `${j.state}:${j.done}`).join(',');
    if (!stamp) return;
    clearTimeout(timer);
    timer = setTimeout(load, 250);
  });

  const Result = $derived(run ? resultScreen(run.workflow) : null);
</script>

{#if error}
  <div class="narrow">
    <Banner tone="danger" title="We couldn't open this activity">{error}</Banner>
    <div class="back"><Button icon={ArrowLeft} onclick={() => router.go('/')}>Back to home</Button></div>
  </div>
{:else if !run}
  <div class="loading"><Spinner size={24} /> <span>Loading…</span></div>
{:else if Result}
  <Result {run} />
{:else}
  <div class="narrow">
    <Banner tone="warning" title="This activity can't be shown">It was made by a newer version of the app.</Banner>
  </div>
{/if}

<style>
  .narrow {
    max-width: 640px;
    margin: 2rem auto;
  }
  .back {
    margin-top: 1rem;
  }
  .loading {
    display: flex;
    gap: 0.75rem;
    align-items: center;
    justify-content: center;
    padding: 4rem;
    color: var(--text-secondary);
  }
</style>
