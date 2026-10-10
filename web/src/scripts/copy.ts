// Copy buttons of the command blocks: copy to the clipboard and flash the
// braille label. Runs after every navigation, so new buttons are wired too.

export function startCopy(): void {
  document.querySelectorAll<HTMLButtonElement>('.term .copy').forEach((button) => {
    if (button.dataset.wired === '1') return;
    button.dataset.wired = '1';
    button.addEventListener('click', async () => {
      try {
        await navigator.clipboard.writeText(button.dataset.copy ?? '');
      } catch {
        /* the clipboard is blocked: the command stays visible anyway */
      }
      button.dataset.copied = '1';
      const label = button.querySelector('.copy-label');
      if (label) label.textContent = '⣿ copied';
      setTimeout(() => {
        button.dataset.copied = '0';
        if (label) label.textContent = '⣿ copy';
      }, 1200);
    });
  });
}
