// Braille rules: a ⣀ fill that always matches the width of its row, exactly
// like the rule the program draws under its headings.

export function fillRules(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('.rule-b[data-fill]').forEach((el) => {
    const wanted = Math.max(1, (Number(el.dataset.fill) || 0) - 2);
    el.textContent = '⣀'.repeat(Math.min(wanted, measure(el)));
  });
}

function measure(el: HTMLElement): number {
  const probe = document.createElement('span');
  probe.style.cssText = 'position:absolute;visibility:hidden;white-space:pre;font:inherit';
  probe.textContent = '⣀';
  el.appendChild(probe);
  const width = probe.getBoundingClientRect().width || 8;
  probe.remove();
  return Math.max(4, Math.floor(el.getBoundingClientRect().width / width));
}
