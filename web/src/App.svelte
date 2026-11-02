<script lang="ts">
  import Header from './components/Header.svelte';
  import Spinner from './components/Spinner.svelte';
  import Toast from './components/Toast.svelte';
  import Closed from './screens/Closed.svelte';
  import Connect from './screens/Connect.svelte';
  import Home from './screens/Home.svelte';
  import RunView from './screens/RunView.svelte';
  import Blocked from './screens/Blocked.svelte';
  import { app, router } from './lib/state.svelte';
  import { workflowScreen } from './workflows/registry';

  // Routes: /  ·  /w/<workflow>/<action>  ·  /runs/<id>
  const seg = $derived(router.segments);
  const needsLink = $derived(['unlinked', 'linking', 'logged_out'].includes(app.status.state));
  const blocked = $derived(['outdated', 'banned', 'replaced'].includes(app.status.state));
  const Wizard = $derived(seg[0] === 'w' ? workflowScreen(seg[1]) : null);
</script>

{#if !app.hasToken}
  <Closed reason="no-token" />
{:else if app.quit}
  <Closed reason="quit" />
{:else if app.serverDown}
  <Closed reason="down" />
{:else if app.loadError}
  <Closed reason="error" message={app.loadError} />
{:else if !app.info || app.status.state === 'starting'}
  <div class="loading">
    <Spinner size={28} />
    <p>Opening the app…</p>
  </div>
{:else}
  <Header />
  <main>
    {#if blocked}
      <Blocked />
    {:else if seg[0] === 'runs' && seg[1]}
      {#key seg[1]}<RunView id={seg[1]} />{/key}
    {:else if needsLink}
      <Connect />
    {:else if Wizard}
      <Wizard action={seg[2] ?? ''} />
    {:else}
      <Home />
    {/if}
  </main>
{/if}
<Toast />

<style>
  main {
    max-width: 1120px;
    margin: 0 auto;
    padding: 2.25rem 1.5rem 4rem;
  }
  .loading {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1rem;
    color: var(--text-secondary);
  }
  @media (max-width: 640px) {
    main {
      padding: 1.5rem 1rem 3rem;
    }
  }
</style>
