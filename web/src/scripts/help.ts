// The keyboard map dialog (?). It only opens and closes: the content is markup.

function el(): HTMLElement | null {
  return document.querySelector<HTMLElement>('[data-help]');
}

export function helpOpen(): boolean {
  const dialog = el();
  return Boolean(dialog && !dialog.hidden);
}

export function openHelp(): void {
  const dialog = el();
  if (!dialog) return;
  dialog.hidden = false;
  document.body.classList.add('lock');
}

export function closeHelp(): void {
  const dialog = el();
  if (!dialog || dialog.hidden) return;
  dialog.hidden = true;
  document.body.classList.remove('lock');
}

export function startHelp(): void {
  const dialog = el();
  if (!dialog || dialog.dataset.wired === '1') return;
  dialog.dataset.wired = '1';
  dialog.addEventListener('click', (event) => {
    if (event.target === dialog) closeHelp();
  });
  document.querySelectorAll<HTMLElement>('[data-help-open]').forEach((button) => {
    if (button.dataset.wired === '1') return;
    button.dataset.wired = '1';
    button.addEventListener('click', openHelp);
  });
}
