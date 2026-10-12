// Copy buttons. One delegated listener for the whole life of the document:
// anything with data-copy copies that text, and a code frame's button copies
// the code it sits in. The label flashes "copied".

const FLASH = 1400;

export async function copyText(text: string, button?: HTMLElement | null): Promise<void> {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    /* the clipboard is blocked: the text stays visible anyway */
  }
  if (!button) return;
  const label = button.querySelector<HTMLElement>('.copy-l');
  const was = label?.textContent ?? '';
  button.dataset.copied = '1';
  if (label) label.textContent = 'copied';
  setTimeout(() => {
    button.dataset.copied = '0';
    if (label) label.textContent = was;
  }, FLASH);
}

let wired = false;

export function startCopy(): void {
  if (wired) return;
  wired = true;
  document.addEventListener('click', (event) => {
    const target = event.target as HTMLElement | null;
    const button = target?.closest<HTMLElement>('[data-copy], [data-copy-code], [data-copy-url]');
    if (!button) return;
    if (button.dataset.copyUrl !== undefined) {
      void copyText(location.href, button);
      return;
    }
    if (button.dataset.copyCode !== undefined) {
      const code = button.closest('pre')?.querySelector('code');
      void copyText(code?.innerText.replace(/\n$/, '') ?? '', button);
      return;
    }
    void copyText(button.dataset.copy ?? '', button);
  });
}
