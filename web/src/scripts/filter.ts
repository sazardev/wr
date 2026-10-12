// The live filter of the reference pages: every row that does not contain the
// words typed is hidden, and so is a group left with no rows.

export function startFilter(): void {
  const input = document.querySelector<HTMLInputElement>('[data-filter-input]');
  if (!input || input.dataset.ready === '1') return;
  input.dataset.ready = '1';
  const rows = [...document.querySelectorAll<HTMLElement>('[data-filter-row]')];
  const groups = [...document.querySelectorAll<HTMLElement>('[data-filter-group]')];
  const count = document.querySelector<HTMLElement>('[data-filter-count]');
  const empty = document.querySelector<HTMLElement>('[data-filter-empty]');
  const text = rows.map((row) => row.textContent!.toLowerCase().replace(/\s+/g, ' '));

  const apply = (): void => {
    const words = input.value.toLowerCase().split(/\s+/).filter(Boolean);
    let shown = 0;
    rows.forEach((row, i) => {
      const match = words.every((word) => text[i]!.includes(word));
      row.hidden = !match;
      if (match) shown++;
    });
    for (const group of groups) {
      group.hidden = ![...group.querySelectorAll<HTMLElement>('[data-filter-row]')].some((row) => !row.hidden);
    }
    if (count) count.textContent = words.length ? `${shown} of ${rows.length}` : `${rows.length}`;
    if (empty) empty.hidden = shown > 0;
  };
  input.addEventListener('input', apply);
  const initial = new URLSearchParams(location.search).get('q');
  if (initial) input.value = initial;
  apply();
}
