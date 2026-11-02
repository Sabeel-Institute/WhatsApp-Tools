<script lang="ts">
  import { onMount } from 'svelte';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import Smartphone from '@lucide/svelte/icons/smartphone';
  import QrCode from '@lucide/svelte/icons/qr-code';
  import CircleCheck from '@lucide/svelte/icons/circle-check';
  import Button from '../components/Button.svelte';
  import Banner from '../components/Banner.svelte';
  import Spinner from '../components/Spinner.svelte';
  import Select from '../components/Select.svelte';
  import TextField from '../components/TextField.svelte';
  import { api, ApiError } from '../lib/api';
  import { app } from '../lib/state.svelte';
  import type { Country } from '../lib/types';

  type Mode = 'qr' | 'phone';
  let mode = $state<Mode>('qr');
  let starting = $state(false);
  let startError = $state<string | null>(null);
  let qrSvg = $state<string | null>(null);
  let expired = $state(false);
  let linkError = $state<string | null>(null);
  let linked = $state(false);

  // Phone number (pairing code) mode
  let countries = $state<Country[]>([]);
  let country = $state('US');
  let phone = $state('');
  let phoneError = $state<string | null>(null);
  let code = $state<string | null>(null);
  let codeFor = $state('');

  const reasons: Record<string, string> = {
    network: "We couldn't reach WhatsApp. Check that this computer is connected to the internet, then try again.",
    outdated: 'WhatsApp needs a newer version of this app. Please ask for the latest version, then try again.',
    no_multi_device:
      "The phone's WhatsApp needs updating before it can link a computer. Update WhatsApp on the phone, then scan the same code again.",
    passkey:
      'This WhatsApp account asks for a passkey before linking a computer, which this app can’t do. Try “Use a phone number instead”.',
    pairing_failed: "Linking didn't finish. Please try again with a new code.",
    unexpected: 'Something unexpected happened while linking. Please try again.',
  };

  async function startQR() {
    starting = true;
    startError = null;
    linkError = null;
    expired = false;
    qrSvg = null;
    code = null;
    try {
      await api.post('/api/wa/login/qr');
    } catch (e) {
      startError = (e as ApiError).message;
    } finally {
      starting = false;
    }
  }

  async function startPhone(e?: Event) {
    e?.preventDefault();
    phoneError = null;
    linkError = null;
    expired = false;
    if (!phone.trim()) {
      phoneError = 'Please type the phone number of the WhatsApp account.';
      return;
    }
    starting = true;
    try {
      const res = await api.post<{ code: string; phone: string }>('/api/wa/login/phone', { phone, country });
      code = res.code;
      codeFor = res.phone;
    } catch (err) {
      const ae = err as ApiError;
      if (ae.code === 'bad_phone') phoneError = ae.message;
      else linkError = ae.message;
    } finally {
      starting = false;
    }
  }

  function switchMode(m: Mode) {
    mode = m;
    linkError = null;
    expired = false;
    if (m === 'qr') startQR();
    else {
      api.post('/api/wa/login/cancel').catch(() => {});
      code = null;
    }
  }

  // Follow login events from the server.
  $effect(() => {
    const ev = app.login;
    if (!ev) return;
    switch (ev.kind) {
      case 'qr':
        if (mode === 'qr') {
          qrSvg = ev.qrSvg ?? null;
          expired = false;
        }
        break;
      case 'pair_code':
        if (mode === 'phone') code = ev.code ?? code;
        break;
      case 'expired':
        expired = true;
        break;
      case 'error':
        linkError = reasons[ev.reason ?? 'unexpected'] ?? reasons.unexpected;
        if (ev.reason !== 'no_multi_device') qrSvg = null;
        break;
      case 'success':
        linked = true;
        break;
    }
  });

  onMount(() => {
    app.login = null;
    startQR();
    api
      .get<Country[]>('/api/countries')
      .then((c) => (countries = c))
      .catch(() => {});
    api
      .get<{ defaultCountry: string }>('/api/settings')
      .then((s) => (country = s.defaultCountry))
      .catch(() => {});
  });

  const loggedOut = $derived(app.status.state === 'logged_out' || app.wasLoggedOut);
</script>

<div class="page">
  {#if loggedOut}
    <div class="notice">
      <Banner tone="warning" title="WhatsApp disconnected this computer" live>
        This happens when someone removes it under Linked devices on the phone, or after a long time without use.
        Nothing else has changed. Link it again below to carry on.
      </Banner>
    </div>
  {/if}

  <section class="card">
    <div class="intro">
      <img class="logo" src="./logo.png" alt="Sabeel Institute" width="148" />
      <h1>Connect your WhatsApp</h1>
      <p class="lede">
        This app works through a WhatsApp account, the same way WhatsApp Web does. You only need to do this once on this
        computer.
      </p>

      {#if mode === 'qr'}
        <ol class="steps">
          <li><span class="n">1</span><span>Open <strong>WhatsApp</strong> on the phone you want to use.</span></li>
          <li>
            <span class="n">2</span><span
              >Tap <strong>Settings</strong> (on iPhone) or the <strong>⋮</strong> menu (on Android), then
              <strong>Linked devices</strong>.</span
            >
          </li>
          <li><span class="n">3</span><span>Tap <strong>Link a device</strong>.</span></li>
          <li><span class="n">4</span><span>Point the phone at the code on this screen.</span></li>
        </ol>
      {:else}
        <ol class="steps">
          <li><span class="n">1</span><span>Type the phone number of the WhatsApp account below.</span></li>
          <li>
            <span class="n">2</span><span
              >On that phone, open WhatsApp, go to <strong>Linked devices</strong>, and tap
              <strong>Link a device</strong>.</span
            >
          </li>
          <li>
            <span class="n">3</span><span>Tap <strong>Link with phone number instead</strong> and type the code shown here.</span>
          </li>
        </ol>
      {/if}

      <div class="tip">
        <Banner tone="tip" title="Which number should I use?">
          We recommend a secondary WhatsApp number that is an admin of your Community, rather than the Community
          owner's number.
        </Banner>
      </div>
    </div>

    <div class="panel">
      {#if linked}
        <div class="state-box" role="status">
          <span class="ok"><CircleCheck size={40} aria-hidden={true} /></span>
          <p class="big">Linked!</p>
          <p class="muted">Finishing the connection…</p>
          <Spinner size={22} />
        </div>
      {:else if mode === 'qr'}
        <div class="qr-frame" aria-live="polite">
          {#if qrSvg && !linkError}
            <div class="qr" class:faded={expired}>{@html qrSvg}</div>
            {#if expired}
              <div class="overlay">
                <p>This code expired.</p>
                <Button variant="primary" icon={RefreshCw} onclick={startQR}>Get a new code</Button>
              </div>
            {/if}
          {:else if linkError || startError}
            <div class="state-box">
              <p class="err-text">{linkError ?? startError}</p>
              <Button variant="primary" icon={RefreshCw} loading={starting} onclick={startQR}>Try again</Button>
            </div>
          {:else}
            <div class="state-box">
              <Spinner size={28} />
              <p class="muted">Getting a code from WhatsApp…</p>
            </div>
          {/if}
        </div>
        <p class="caption">
          {#if app.info?.demo}
            Demo mode: this pretend code links by itself in a few seconds.
          {:else if qrSvg && !expired}
            The code refreshes automatically. Keep this window open while you scan.
          {:else}
            &nbsp;
          {/if}
        </p>
        <Button variant="ghost" icon={Smartphone} onclick={() => switchMode('phone')}>Use a phone number instead</Button>
      {:else}
        <div class="phone-box">
          {#if code && !expired}
            <p class="muted">Type this code on the phone ({codeFor}):</p>
            <p class="code tabular" aria-label="Pairing code {code.split('').join(' ')}">{code}</p>
            <p class="waiting"><Spinner size={18} /> Waiting for you to enter it on the phone…</p>
            {#if app.info?.demo}<p class="caption">Demo mode: this pretend code links by itself in a few seconds.</p>{/if}
          {:else}
            <form onsubmit={startPhone}>
              {#if expired}
                <Banner tone="warning">That code expired. Get a new one and type it within a couple of minutes.</Banner>
              {/if}
              <Select
                label="Country"
                bind:value={country}
                options={countries.length
                  ? countries.map((c) => ({ value: c.code, label: `${c.name} (${c.dial})` }))
                  : [{ value: country, label: country }]}
              />
              <TextField
                label="WhatsApp phone number"
                bind:value={phone}
                placeholder="(212) 555-0100"
                inputmode="tel"
                autocomplete="tel"
                error={phoneError}
              />
              <Button type="submit" variant="primary" full loading={starting}>Get my code</Button>
            </form>
          {/if}
          {#if linkError}<p class="err-text" role="alert">{linkError}</p>{/if}
        </div>
        <Button variant="ghost" icon={QrCode} onclick={() => switchMode('qr')}>Use a QR code instead</Button>
      {/if}
    </div>
  </section>
</div>

<style>
  .page {
    max-width: 980px;
    margin: 0 auto;
  }
  .notice {
    margin-bottom: 1.25rem;
  }
  .card {
    display: grid;
    grid-template-columns: 1.1fr 0.9fr;
    gap: 2.5rem;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 2.5rem;
    box-shadow: var(--shadow);
  }
  .logo {
    height: auto;
    margin-bottom: 1.5rem;
  }
  h1 {
    font-size: 1.875rem;
  }
  .lede {
    margin-top: 0.6rem;
    color: var(--text-secondary);
    font-size: 1.0625rem;
  }
  .steps {
    list-style: none;
    margin: 1.6rem 0 0;
    padding: 0;
    display: grid;
    gap: 0.85rem;
  }
  .steps li {
    display: flex;
    gap: 0.8rem;
    align-items: flex-start;
  }
  .n {
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
  .tip {
    margin-top: 1.75rem;
  }
  .panel {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.75rem;
    border-left: 1px solid var(--border-subtle);
    padding-left: 2.5rem;
  }
  .qr-frame {
    position: relative;
    width: 300px;
    height: 300px;
    background: #fff;
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 14px;
    display: grid;
    place-items: center;
    box-shadow: var(--shadow-sm);
  }
  .qr {
    width: 100%;
    height: 100%;
    transition: opacity 0.2s ease;
  }
  .qr :global(svg) {
    width: 100%;
    height: 100%;
    display: block;
  }
  .qr.faded {
    opacity: 0.12;
  }
  .overlay {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.9rem;
    font-weight: 600;
  }
  .state-box {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.9rem;
    text-align: center;
    padding: 1rem;
  }
  .ok {
    color: var(--success);
  }
  .big {
    font-size: 1.375rem;
    font-weight: 650;
  }
  .caption {
    font-size: 0.875rem;
    color: var(--text-secondary);
    text-align: center;
    max-width: 300px;
    min-height: 1.4em;
  }
  .err-text {
    color: var(--danger);
    font-size: 0.9375rem;
  }
  .phone-box {
    width: 100%;
    max-width: 340px;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    text-align: center;
  }
  form {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    text-align: left;
  }
  .code {
    font-family: var(--font-mono);
    font-size: 2.4rem;
    font-weight: 600;
    letter-spacing: 0.12em;
    color: var(--text-primary);
    background: #fff;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 1rem 0.5rem;
  }
  .waiting {
    display: flex;
    gap: 0.5rem;
    align-items: center;
    justify-content: center;
    color: var(--text-secondary);
    font-size: 0.9375rem;
  }
  @media (max-width: 900px) {
    .card {
      grid-template-columns: 1fr;
      padding: 1.75rem;
    }
    .panel {
      border-left: none;
      padding-left: 0;
      border-top: 1px solid var(--border-subtle);
      padding-top: 2rem;
    }
  }
  @media (max-width: 420px) {
    .qr-frame {
      width: 260px;
      height: 260px;
    }
  }
</style>
