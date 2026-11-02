// A single, polite toast for confirmations like "Copied".
class Toast {
  message = $state<string | null>(null);
  private timer: ReturnType<typeof setTimeout> | undefined;

  show(message: string) {
    this.message = message;
    clearTimeout(this.timer);
    this.timer = setTimeout(() => (this.message = null), 2600);
  }
}

export const toast = new Toast();

export async function copyText(text: string, what = 'Copied'): Promise<void> {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    // Clipboard API refused (rare on localhost): fall back to a hidden field.
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.append(ta);
    ta.select();
    document.execCommand('copy');
    ta.remove();
  }
  toast.show(what);
}
