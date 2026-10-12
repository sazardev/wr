// "Noise vs. signal": the clutter of a typical page is struck out and removed
// one piece at a time the first time the section is seen, and the counter runs
// with it. The button plays it again.

const STEP = 300;

export function startNoise(): void {
  const root = document.querySelector<HTMLElement>('[data-noise]');
  if (!root || root.dataset.ready === '1') return;
  root.dataset.ready = '1';

  const items = [...root.querySelectorAll<HTMLElement>('[data-n]')];
  const count = document.querySelector<HTMLElement>('[data-noise-count]');
  const replay = document.querySelector<HTMLElement>('[data-noise-replay]');
  let timers: ReturnType<typeof setTimeout>[] = [];

  const reset = (): void => {
    timers.forEach(clearTimeout);
    timers = [];
    items.forEach((item) => item.classList.remove('struck', 'is-gone'));
    if (count) count.textContent = `0/${items.length}`;
  };

  const play = (): void => {
    reset();
    items.forEach((item, i) => {
      timers.push(
        setTimeout(() => {
          item.classList.add('struck');
          if (count) count.textContent = `${i + 1}/${items.length}`;
        }, 500 + i * STEP),
        setTimeout(() => item.classList.add('is-gone'), 500 + i * STEP + 320),
      );
    });
  };

  replay?.addEventListener('click', play);
  if (!('IntersectionObserver' in window)) return play();
  const io = new IntersectionObserver(
    (entries) => {
      if (!entries[0]!.isIntersecting) return;
      io.disconnect();
      play();
    },
    { threshold: 0.45 },
  );
  io.observe(root);
}
