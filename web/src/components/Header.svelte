<script lang="ts">
  import Power from '@lucide/svelte/icons/power';
  import LogOut from '@lucide/svelte/icons/log-out';
  import FlaskConical from '@lucide/svelte/icons/flask-conical';
  import { app, router } from '../lib/state.svelte';
  import { api, ApiError } from '../lib/api';
  import { initials } from '../lib/format';
  import Button from './Button.svelte';
  import Modal from './Modal.svelte';

  let confirmQuit = $state(false);
  let confirmDisconnect = $state(false);
  let busy = $state(false);
  let error = $state<string | null>(null);
  let menuOpen = $state(false);
  let menuEl: HTMLDivElement | undefined = $state();

  const account = $derived(app.status.account);
  const jobRunning = $derived(!!app.activeJob);

  async function quit() {
    busy = true;
    error = null;
    try {
      await api.post('/api/quit');
      app.quit = true;
      confirmQuit = false;
    } catch (e) {
      error = (e as ApiError).message;
    } finally {
      busy = false;
    }
  }

  async function disconnect() {
    busy = true;
    error = null;
    try {
      await api.post('/api/wa/logout');
      confirmDisconnect = false;
      router.go('/');
    } catch (e) {
      error = (e as ApiError).message;
    } finally {
      busy = false;
    }
  }

  function onWindowClick(e: MouseEvent) {
    if (menuOpen && menuEl && !menuEl.contains(e.target as Node)) menuOpen = false;
  }
</script>

<svelte:window onclick={onWindowClick} onkeydown={(e) => e.key === 'Escape' && (menuOpen = false)} />

<header>
  <div class="inner">
    <a class="brand" href="#/" aria-label="Home">
      <img src="./mark.png" alt="" width="54" height="33" />
      <span class="name">WhatsApp Tools</span>
    </a>

    <div class="right">
      {#if app.info?.demo}
        <span class="demo" title="Nothing is sent to WhatsApp in demo mode">
          <FlaskConical size={15} aria-hidden={true} /> Demo mode
        </span>
      {/if}

      {#if account && app.status.state !== 'unlinked' && app.status.state !== 'logged_out'}
        <div class="account" bind:this={menuEl}>
          <button
            class="account-btn"
            aria-haspopup="true"
            aria-expanded={menuOpen}
            onclick={() => (menuOpen = !menuOpen)}
          >
            <span class="avatar" aria-hidden="true">{initials(account.name || 'WhatsApp')}</span>
            <span class="who">
              <span class="acct-name">{account.name || 'WhatsApp account'}</span>
              <span class="acct-phone tabular">
                {#if app.status.state === 'connected'}
                  <span class="dot ok" aria-hidden="true"></span>Connected
                {:else}
                  <span class="dot wait" aria-hidden="true"></span>Reconnecting…
                {/if}
              </span>
            </span>
          </button>
          {#if menuOpen}
            <div class="menu" role="menu">
              <p class="menu-head">
                <span class="menu-name">{account.name || 'WhatsApp account'}</span>
                <span class="tabular">{account.display || account.phone}</span>
              </p>
              <button
                role="menuitem"
                class="menu-item"
                onclick={() => {
                  menuOpen = false;
                  error = null;
                  confirmDisconnect = true;
                }}
              >
                <LogOut size={17} aria-hidden={true} /> Disconnect WhatsApp…
              </button>
            </div>
          {/if}
        </div>
      {/if}

      <Button
        variant="quiet"
        size="sm"
        icon={Power}
        onclick={() => {
          error = null;
          confirmQuit = true;
        }}>Quit</Button
      >
    </div>
  </div>
</header>

<Modal
  open={confirmQuit}
  title={jobRunning ? 'The app is still working' : 'Quit the app?'}
  onclose={() => (confirmQuit = false)}
>
  {#if jobRunning}
    <p>
      “{app.activeJob?.title}” is still in progress. Quitting now would leave it unfinished, so please wait until it
      finishes, or stop it first.
    </p>
  {:else}
    <p>WhatsApp stays connected for next time, so you won't need to scan the code again.</p>
  {/if}
  {#if error}<p class="err">{error}</p>{/if}
  {#snippet actions()}
    {#if jobRunning}
      <Button variant="primary" onclick={() => (confirmQuit = false)}>OK</Button>
    {:else}
      <Button onclick={() => (confirmQuit = false)}>Cancel</Button>
      <Button variant="primary" loading={busy} onclick={quit}>Quit</Button>
    {/if}
  {/snippet}
</Modal>

<Modal open={confirmDisconnect} title="Disconnect WhatsApp?" onclose={() => (confirmDisconnect = false)}>
  <p>
    This computer will be removed from the account's linked devices. Your groups and messages are not affected. To use
    the app again, you'll scan a QR code.
  </p>
  {#if jobRunning}
    <p class="err">Please wait for “{app.activeJob?.title}” to finish first.</p>
  {/if}
  {#if error}<p class="err">{error}</p>{/if}
  {#snippet actions()}
    <Button onclick={() => (confirmDisconnect = false)}>Cancel</Button>
    <Button variant="danger" loading={busy} disabled={jobRunning} onclick={disconnect}>Disconnect</Button>
  {/snippet}
</Modal>

<style>
  /* Full-page screenshots (docs) keep the header at the top. */
  :global(html[data-capture]) header {
    position: relative;
  }
  header {
    position: sticky;
    top: 0;
    z-index: 20;
    background: rgba(246, 235, 221, 0.92);
    backdrop-filter: saturate(1.2) blur(8px);
    border-bottom: 1px solid var(--border-subtle);
  }
  .inner {
    max-width: 1120px;
    margin: 0 auto;
    padding: 0.6rem 1.5rem;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    min-height: 64px;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    text-decoration: none;
    color: var(--text-primary);
    border-radius: var(--radius-sm);
  }
  .brand img {
    display: block;
    height: 33px;
    width: auto;
  }
  .name {
    font-weight: 650;
    font-size: 1.0625rem;
    letter-spacing: -0.01em;
    padding-left: 0.75rem;
    border-left: 1px solid var(--gold);
  }
  .right {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }
  .demo {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--warning-text);
    background: var(--warning-soft);
    border: 1px solid #e6d3a8;
    padding: 0.25rem 0.65rem;
    border-radius: 999px;
  }
  .account {
    position: relative;
  }
  .account-btn {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    background: none;
    border: 1px solid transparent;
    border-radius: 999px;
    padding: 0.25rem 0.75rem 0.25rem 0.25rem;
    cursor: pointer;
    text-align: left;
  }
  .account-btn:hover {
    background: var(--bg-hover);
    border-color: var(--border-subtle);
  }
  .avatar {
    width: 36px;
    height: 36px;
    border-radius: 50%;
    background: var(--bg-accent-soft);
    color: var(--accent);
    display: grid;
    place-items: center;
    font-weight: 650;
    font-size: 0.8125rem;
  }
  .who {
    display: flex;
    flex-direction: column;
    line-height: 1.25;
  }
  .acct-name {
    font-weight: 600;
    font-size: 0.875rem;
  }
  .acct-phone {
    font-size: 0.8125rem;
    color: var(--text-secondary);
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
  }
  .dot.ok {
    background: var(--success);
  }
  .dot.wait {
    background: var(--warning);
  }
  .menu {
    position: absolute;
    right: 0;
    top: calc(100% + 6px);
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    box-shadow: var(--shadow-lg);
    min-width: 250px;
    padding: 0.4rem;
  }
  .menu-head {
    display: flex;
    flex-direction: column;
    padding: 0.6rem 0.75rem 0.7rem;
    border-bottom: 1px solid var(--border-subtle);
    margin-bottom: 0.3rem;
    font-size: 0.875rem;
    color: var(--text-secondary);
  }
  .menu-name {
    font-weight: 600;
    color: var(--text-primary);
  }
  .menu-item {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    width: 100%;
    background: none;
    border: none;
    border-radius: var(--radius-sm);
    padding: 0.6rem 0.75rem;
    cursor: pointer;
    color: var(--danger);
    font-weight: 560;
    font-size: 0.9375rem;
  }
  .menu-item:hover {
    background: var(--danger-soft);
  }
  .err {
    color: var(--danger);
    margin-top: 0.6rem;
  }
  @media (max-width: 640px) {
    .who {
      display: none;
    }
    .name {
      display: none;
    }
  }
</style>
