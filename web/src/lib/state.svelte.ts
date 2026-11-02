import { api, streamEvents, token } from './api';
import type { AppInfo, Job, LoginEvent, WAStatus } from './types';

/** App-wide state, kept live by the server's event stream. */
class AppState {
  info = $state<AppInfo | null>(null);
  status = $state<WAStatus>({ state: 'starting', since: '' });
  /** Latest snapshot of every job seen this session, by ID. */
  jobs = $state<Record<string, Job>>({});
  /** The job currently running (or paused/waiting), if any. */
  activeJobId = $state<string | null>(null);
  login = $state<LoginEvent | null>(null);
  /** True once the server stopped answering (the app quit or crashed). */
  serverDown = $state(false);
  /** True after the user chose Quit. */
  quit = $state(false);
  loadError = $state<string | null>(null);
  hasToken = !!token;
  /** Set when WhatsApp unlinked this computer while the app was open. */
  wasLoggedOut = $state(false);

  get activeJob(): Job | null {
    return this.activeJobId ? (this.jobs[this.activeJobId] ?? null) : null;
  }

  get connected(): boolean {
    return this.status.state === 'connected';
  }

  async init() {
    if (!this.hasToken) return;
    try {
      this.info = await api.get<AppInfo>('/api/app');
      this.status = await api.get<WAStatus>('/api/wa/status');
      const { job } = await api.get<{ job: Job | null }>('/api/jobs/active');
      if (job) this.setJob(job);
    } catch (e) {
      this.loadError = (e as Error).message;
      return;
    }
    streamEvents(
      (type, data) => this.onEvent(type, data),
      () => (this.serverDown = false),
      () => (this.serverDown = true),
    );
    // A heartbeat keeps the app running while this tab is open.
    setInterval(() => {
      if (!this.quit) api.post('/api/heartbeat').catch(() => {});
    }, 60_000);
  }

  setJob(job: Job) {
    this.jobs[job.id] = job;
    const finished = job.state === 'completed' || job.state === 'stopped' || job.state === 'failed';
    if (!finished) this.activeJobId = job.id;
    else if (this.activeJobId === job.id) this.activeJobId = null;
  }

  private onEvent(type: string, data: unknown) {
    switch (type) {
      case 'status': {
        const next = data as WAStatus;
        if (next.state === 'logged_out' && this.status.state !== 'logged_out') this.wasLoggedOut = true;
        if (next.state === 'connected') this.wasLoggedOut = false;
        this.status = next;
        break;
      }
      case 'login':
        this.login = data as LoginEvent;
        break;
      case 'job':
        this.setJob(data as Job);
        break;
    }
  }
}

export const app = new AppState();

/** Minimal hash router: #/path/segments */
class Router {
  path = $state(normalize(location.hash));

  constructor() {
    window.addEventListener('hashchange', () => {
      this.path = normalize(location.hash);
      window.scrollTo({ top: 0 });
    });
  }

  go(path: string) {
    if (normalize('#' + path) === this.path) return;
    location.hash = '#' + path;
  }

  get segments(): string[] {
    return this.path.split('/').filter(Boolean);
  }
}

function normalize(hash: string): string {
  const p = hash.replace(/^#/, '');
  return p.startsWith('/') ? p : '/';
}

export const router = new Router();
