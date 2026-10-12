// The install box: tabs switch the command, the note and what the copy button
// copies. The first tab is server-rendered, so it works without this.

export function startInstall(): void {
  document.querySelectorAll<HTMLElement>('[data-install]').forEach((box) => {
    if (box.dataset.ready === '1') return;
    box.dataset.ready = '1';
    const tabs = [...box.querySelectorAll<HTMLButtonElement>('[role="tab"]')];
    const cmd = box.querySelector<HTMLElement>('[data-install-cmd]');
    const note = box.querySelector<HTMLElement>('[data-install-note]');
    const copy = box.querySelector<HTMLElement>('[data-copy]');

    const select = (tab: HTMLButtonElement, focus = false): void => {
      for (const other of tabs) {
        other.setAttribute('aria-selected', String(other === tab));
        other.tabIndex = other === tab ? 0 : -1;
      }
      if (cmd) cmd.textContent = tab.dataset.cmd ?? '';
      if (note) note.textContent = tab.dataset.note ?? '';
      if (copy) copy.dataset.copy = tab.dataset.cmd ?? '';
      if (focus) tab.focus();
    };

    tabs.forEach((tab, i) => {
      tab.tabIndex = i === 0 ? 0 : -1;
      tab.addEventListener('click', () => select(tab));
      tab.addEventListener('keydown', (event) => {
        const step = event.key === 'ArrowRight' ? 1 : event.key === 'ArrowLeft' ? -1 : 0;
        if (!step) return;
        event.preventDefault();
        select(tabs[(i + step + tabs.length) % tabs.length]!, true);
      });
    });
  });
}
