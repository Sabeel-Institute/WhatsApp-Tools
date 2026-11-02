// Talking to the Go server. Every request carries the session token the app
// put in the URL it opened (#token=...), which is moved to sessionStorage
// and removed from the address bar on load.

const TOKEN_KEY = 'sabeel-wa-token';

function readToken(): string {
  const m = location.hash.match(/token=([A-Za-z0-9]+)/);
  if (m) {
    try {
      sessionStorage.setItem(TOKEN_KEY, m[1]);
    } catch {
      /* private mode: keep it in memory only */
    }
    history.replaceState(null, '', location.pathname + location.search + '#/');
    return m[1];
  }
  try {
    return sessionStorage.getItem(TOKEN_KEY) ?? '';
  } catch {
    return '';
  }
}

export const token = readToken();

export class ApiError extends Error {
  status: number;
  code: string;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

const OFFLINE_MESSAGE = "The app isn't running any more. Open it again from your apps menu.";

async function request<T>(method: string, path: string, body?: unknown, raw?: BodyInit): Promise<T> {
  const headers: Record<string, string> = { 'X-App-Token': token };
  let payload: BodyInit | undefined = raw;
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
    payload = JSON.stringify(body);
  }
  let res: Response;
  try {
    res = await fetch(path, { method, headers, body: payload, cache: 'no-store' });
  } catch {
    throw new ApiError(0, 'offline', OFFLINE_MESSAGE);
  }
  if (res.status === 401) {
    throw new ApiError(401, 'unauthorized', 'Please open this page from the app, not from a bookmark.');
  }
  const text = await res.text();
  let data: unknown = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    /* not JSON */
  }
  if (!res.ok) {
    const err = (data as { error?: { code: string; message: string } } | null)?.error;
    throw new ApiError(res.status, err?.code ?? 'error', err?.message ?? 'Something went wrong. Please try again.');
  }
  return data as T;
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body: unknown = {}) => request<T>('POST', path, body),
  put: <T>(path: string, body: unknown) => request<T>('PUT', path, body),
  upload: <T>(path: string, file: Blob) => request<T>('POST', path, undefined, file),
};

/** Downloads a file from the API (which needs the token header). */
export async function download(path: string, fallbackName: string): Promise<void> {
  let res: Response;
  try {
    res = await fetch(path, { headers: { 'X-App-Token': token } });
  } catch {
    throw new ApiError(0, 'offline', OFFLINE_MESSAGE);
  }
  if (!res.ok) throw new ApiError(res.status, 'download', "We couldn't create the file. Please try again.");
  const blob = await res.blob();
  const cd = res.headers.get('Content-Disposition') ?? '';
  const m = cd.match(/filename\*=UTF-8''([^;]+)/);
  const name = m ? decodeURIComponent(m[1]) : fallbackName;
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = name;
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 10_000);
}

export type EventHandler = (type: string, data: unknown) => void;

/**
 * Streams server-sent events (via fetch, so the token can be a header) and
 * reconnects with backoff. onDown is called when the server can't be reached.
 */
export function streamEvents(onEvent: EventHandler, onUp: () => void, onDown: () => void): () => void {
  let stopped = false;
  let controller: AbortController | null = null;
  let failures = 0;

  async function loop() {
    while (!stopped) {
      controller = new AbortController();
      try {
        const res = await fetch('/api/events', {
          headers: { 'X-App-Token': token },
          signal: controller.signal,
          cache: 'no-store',
        });
        if (!res.ok || !res.body) throw new Error(`events: ${res.status}`);
        failures = 0;
        onUp();
        const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
        let buf = '';
        for (;;) {
          const { value, done } = await reader.read();
          if (done) break;
          buf += value;
          let idx: number;
          while ((idx = buf.indexOf('\n\n')) >= 0) {
            const chunk = buf.slice(0, idx);
            buf = buf.slice(idx + 2);
            let type = 'message';
            let data = '';
            for (const line of chunk.split('\n')) {
              if (line.startsWith('event: ')) type = line.slice(7);
              else if (line.startsWith('data: ')) data += line.slice(6);
            }
            if (data) {
              try {
                onEvent(type, JSON.parse(data));
              } catch {
                /* ignore a malformed event */
              }
            }
          }
        }
      } catch {
        if (stopped) return;
      }
      failures++;
      if (failures >= 2) onDown();
      await new Promise((r) => setTimeout(r, Math.min(1000 * failures, 5000)));
    }
  }
  loop();
  return () => {
    stopped = true;
    controller?.abort();
  };
}
