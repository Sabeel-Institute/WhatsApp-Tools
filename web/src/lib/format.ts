// Formatting helpers for people, not machines.

const nf = new Intl.NumberFormat('en-US');

export const num = (n: number) => nf.format(n);

/** "1 person" / "3 people" */
export function people(n: number): string {
  return `${num(n)} ${n === 1 ? 'person' : 'people'}`;
}

export function plural(n: number, one: string, many: string): string {
  return `${num(n)} ${n === 1 ? one : many}`;
}

/** "Today, 3:42 PM", "Yesterday, 9:05 AM", "Mon, Oct 27, 2:10 PM" */
export function when(iso: string): string {
  const d = new Date(iso);
  const now = new Date();
  const time = d.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
  const day = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const diff = Math.round((day(now) - day(d)) / 86_400_000);
  if (diff === 0) return `Today, ${time}`;
  if (diff === 1) return `Yesterday, ${time}`;
  const sameYear = d.getFullYear() === now.getFullYear();
  const date = d.toLocaleDateString(undefined, {
    weekday: diff < 7 ? 'short' : undefined,
    month: 'short',
    day: 'numeric',
    year: sameYear ? undefined : 'numeric',
  });
  return `${date}, ${time}`;
}

/** "about 2 minutes", "less than a minute" */
export function duration(seconds: number): string {
  if (seconds < 45) return 'less than a minute';
  if (seconds < 90) return 'about a minute';
  const m = Math.round(seconds / 60);
  if (m < 60) return `about ${m} minutes`;
  const h = Math.floor(m / 60);
  const rest = m % 60;
  return `about ${h} hour${h > 1 ? 's' : ''}${rest >= 10 ? ` ${rest} minutes` : ''}`;
}

/** Initials for an avatar. */
export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return '?';
  return (parts[0][0] + (parts.length > 1 ? parts[parts.length - 1][0] : '')).toUpperCase();
}

/** UTF-16 length, which is how WhatsApp counts group-name characters. */
export const textLength = (s: string) => s.length;
