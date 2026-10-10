// The spinner glyphs of the "terminal" chrome: dots, status bars and titles.

const SPIN = ['⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'];

export function startSpin(): void {
  const dots = document.querySelectorAll<HTMLElement>('[data-spin]');
  if (!dots.length) return;
  let i = 0;
  setInterval(() => {
    i = (i + 1) % SPIN.length;
    for (const dot of dots) dot.textContent = SPIN[i]!;
  }, 110);
}
