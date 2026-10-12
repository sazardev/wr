// The product window of the landing page: the recorded scenes, one tab each,
// advancing on their own with a progress line, pausing under the pointer, and
// the command that produced the frame typed underneath. The frames and their
// commands are data in the HTML (data-name, data-cmd); ?scene= opens one.

const HOLD = 5200;
const TYPE_AT = 32;

export function startReel(): void {
  const reel = document.getElementById('reel');
  if (!reel || reel.dataset.ready === '1') return;
  reel.dataset.ready = '1';

  const frames = [...reel.querySelectorAll<HTMLImageElement>('.win-frame')];
  const tabs = [...reel.querySelectorAll<HTMLButtonElement>('[role="tab"]')];
  const typed = reel.querySelector<HTMLElement>('[data-reel-type]');
  const prog = reel.querySelector<HTMLElement>('.win-prog');
  if (!frames.length) return;

  let current = 0;
  let typer: ReturnType<typeof setInterval> | undefined;
  let advance: ReturnType<typeof setTimeout> | undefined;
  let paused = false;
  let visible = true;

  const restartBar = (): void => {
    if (!prog) return;
    prog.classList.remove('run');
    void prog.offsetWidth;
    if (!paused && visible) prog.classList.add('run');
  };

  const schedule = (): void => {
    clearTimeout(advance);
    if (paused || !visible) return;
    advance = setTimeout(() => show((current + 1) % frames.length), HOLD);
  };

  const show = (index: number): void => {
    current = index;
    frames.forEach((frame, i) => frame.classList.toggle('is-on', i === index));
    tabs.forEach((tab, i) => {
      tab.setAttribute('aria-selected', String(i === index));
      tab.tabIndex = i === index ? 0 : -1;
    });
    const cmd = frames[index]!.dataset.cmd ?? '';
    clearInterval(typer);
    if (typed) {
      typed.textContent = '';
      let n = 0;
      typer = setInterval(() => {
        typed.textContent = cmd.slice(0, ++n);
        if (n >= cmd.length) clearInterval(typer);
      }, TYPE_AT);
    }
    restartBar();
    schedule();
  };

  tabs.forEach((tab, i) => {
    tab.addEventListener('click', () => show(i));
    tab.addEventListener('keydown', (event) => {
      const step = event.key === 'ArrowRight' ? 1 : event.key === 'ArrowLeft' ? -1 : 0;
      if (!step) return;
      event.preventDefault();
      const next = (i + step + tabs.length) % tabs.length;
      show(next);
      tabs[next]!.focus();
    });
  });

  const pause = (on: boolean): void => {
    paused = on;
    if (on) {
      clearTimeout(advance);
      prog?.classList.remove('run');
    } else restartBar(), schedule();
  };
  reel.addEventListener('pointerenter', () => pause(true));
  reel.addEventListener('pointerleave', () => pause(false));
  reel.addEventListener('focusin', () => pause(true));
  reel.addEventListener('focusout', () => pause(false));

  if ('IntersectionObserver' in window) {
    new IntersectionObserver((entries) => {
      visible = entries[0]!.isIntersecting && !document.hidden;
      if (visible) restartBar(), schedule();
      else clearTimeout(advance);
    }).observe(reel);
  }

  const asked = new URLSearchParams(location.search).get('scene');
  show(Math.max(0, frames.findIndex((frame) => frame.dataset.name === asked)));
}
