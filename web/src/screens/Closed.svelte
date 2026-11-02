<script lang="ts">
  import Power from '@lucide/svelte/icons/power';
  import Unplug from '@lucide/svelte/icons/unplug';
  import KeyRound from '@lucide/svelte/icons/key-round';
  import CircleAlert from '@lucide/svelte/icons/circle-alert';

  let { reason, message }: { reason: 'quit' | 'down' | 'no-token' | 'error'; message?: string } = $props();
</script>

<div class="wrap">
  <div class="card">
    <img src="./logo.png" alt="Sabeel Institute" width="180" />
    {#if reason === 'quit'}
      <span class="icon"><Power size={22} aria-hidden={true} /></span>
      <h1>The app has closed</h1>
      <p>You can close this browser tab. To use the app again, open “Sabeel WhatsApp Tools” from your apps menu.</p>
    {:else if reason === 'down'}
      <span class="icon"><Unplug size={22} aria-hidden={true} /></span>
      <h1>The app isn't running</h1>
      <p>
        It may have been closed, or the computer went to sleep. Open “Sabeel WhatsApp Tools” from your apps menu, and it
        will open a fresh tab. You can close this one.
      </p>
    {:else if reason === 'no-token'}
      <span class="icon"><KeyRound size={22} aria-hidden={true} /></span>
      <h1>Please open the app from your apps menu</h1>
      <p>
        For your security, this page only works in the tab the app opens itself. Open “Sabeel WhatsApp Tools” from your
        apps menu or desktop, and it will open the right page.
      </p>
    {:else}
      <span class="icon"><CircleAlert size={22} aria-hidden={true} /></span>
      <h1>Something went wrong</h1>
      <p>{message} Try closing this tab and opening the app again.</p>
    {/if}
  </div>
</div>

<style>
  .wrap {
    min-height: 100vh;
    display: grid;
    place-items: center;
    padding: 2rem 1rem;
  }
  .card {
    max-width: 520px;
    text-align: center;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.9rem;
  }
  img {
    height: auto;
    margin-bottom: 1.5rem;
  }
  .icon {
    width: 48px;
    height: 48px;
    border-radius: 50%;
    display: grid;
    place-items: center;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    color: var(--accent);
  }
  h1 {
    font-size: 1.5rem;
  }
  p {
    color: var(--text-secondary);
  }
</style>
