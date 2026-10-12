// Dresses the rendered Markdown after every navigation: a copy button in every
// code frame, a link on every heading, and a scrolling wrapper on every table.

const COPY_ICON =
  '<svg class="ico" viewBox="0 0 16 16" aria-hidden="true"><rect x="5.5" y="5.5" width="8" height="8"/><path d="M10.5 5.5v-3h-8v8h3"/></svg>';

export function startProse(): void {
  document.querySelectorAll<HTMLPreElement>('.prose pre.astro-code').forEach((pre) => {
    if (pre.dataset.dressed === '1') return;
    pre.dataset.dressed = '1';
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'code-copy';
    button.dataset.copyCode = '';
    button.setAttribute('aria-label', 'Copy code');
    button.innerHTML = `${COPY_ICON}<span class="copy-l">copy</span>`;
    pre.append(button);
  });

  document.querySelectorAll<HTMLElement>('.prose h2[id], .prose h3[id]').forEach((heading) => {
    if (heading.querySelector('.anchor')) return;
    const link = document.createElement('a');
    link.className = 'anchor';
    link.href = `#${heading.id}`;
    link.textContent = '#';
    link.setAttribute('aria-label', `Link to “${heading.textContent}”`);
    heading.append(link);
  });

  document.querySelectorAll<HTMLTableElement>('.prose > table').forEach((table) => {
    const wrap = document.createElement('div');
    wrap.className = 'tw';
    table.replaceWith(wrap);
    wrap.append(table);
  });
}
