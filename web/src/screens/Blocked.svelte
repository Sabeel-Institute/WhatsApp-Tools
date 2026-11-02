<script lang="ts">
  import Download from '@lucide/svelte/icons/download';
  import ShieldAlert from '@lucide/svelte/icons/shield-alert';
  import MonitorSmartphone from '@lucide/svelte/icons/monitor-smartphone';
  import { app } from '../lib/state.svelte';

  const state = $derived(app.status.state);
</script>

<section class="card" aria-live="polite">
  {#if state === 'outdated'}
    <span class="icon warn"><Download size={24} aria-hidden={true} /></span>
    <h1>Please update the app</h1>
    <p>
      WhatsApp has changed and needs a newer version of Sabeel WhatsApp Tools before it will connect again. Nothing is
      wrong with your account.
    </p>
    <ol>
      <li>
        Ask the person who looks after this app for the latest version, or download it from the project's Releases page.
      </li>
      <li>Quit this app, replace it with the new version, and open it again.</li>
      <li>You won't need to scan the code again: the connection is kept.</li>
    </ol>
  {:else if state === 'banned'}
    <span class="icon danger"><ShieldAlert size={24} aria-hidden={true} /></span>
    <h1>WhatsApp has paused this account</h1>
    <p>
      WhatsApp temporarily restricted this account{app.status.detail ? ` for ${app.status.detail}` : ''}. This usually
      happens after adding or messaging many people who don't have the number saved.
    </p>
    <ol>
      <li>Wait until the restriction ends. Check the WhatsApp app on the phone for details.</li>
      <li>Afterwards, add people in smaller groups, and ask registrants to save the number first.</li>
    </ol>
  {:else}
    <span class="icon warn"><MonitorSmartphone size={24} aria-hidden={true} /></span>
    <h1>WhatsApp is open somewhere else</h1>
    <p>
      This connection was taken over by another copy of the app using the same link. Only one copy can use it at a time.
    </p>
    <ol>
      <li>Close the app on the other computer.</li>
      <li>Then quit and reopen the app here.</li>
    </ol>
  {/if}
</section>

<style>
  .card {
    max-width: 640px;
    margin: 2rem auto 0;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 2.25rem;
    box-shadow: var(--shadow);
  }
  .icon {
    width: 52px;
    height: 52px;
    border-radius: 50%;
    display: grid;
    place-items: center;
    margin-bottom: 1.1rem;
  }
  .warn {
    background: var(--warning-soft);
    color: var(--warning);
  }
  .danger {
    background: var(--danger-soft);
    color: var(--danger);
  }
  h1 {
    font-size: 1.625rem;
  }
  p {
    margin-top: 0.6rem;
    color: var(--text-secondary);
  }
  ol {
    margin: 1.4rem 0 0;
    padding: 0;
    list-style: none;
    display: grid;
    gap: 0.75rem;
    counter-reset: step;
  }
  li {
    display: flex;
    gap: 0.8rem;
    align-items: flex-start;
    counter-increment: step;
  }
  li::before {
    content: counter(step);
    flex: none;
    width: 28px;
    height: 28px;
    border-radius: 50%;
    background: var(--bg-accent-soft);
    color: var(--accent);
    display: grid;
    place-items: center;
    font-weight: 650;
    font-size: 0.875rem;
  }
</style>
