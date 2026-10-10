// The hero reel: the captured scenes, one at a time, with the command that
// produced it typed underneath. The frames and their commands are data in the
// HTML (data-name, data-cmd); this only drives them. ?scene= opens a specific
// frame, for linking to one moment of the demo.

const HOLD = 4600;
const TYPE_AT = 34;

export function startReel(): void {
  const reel = document.getElementById('reel');
  if (!reel) return;

  const frames = [...reel.querySelectorAll<HTMLImageElement>('.reel-frame')];
  if (!frames.length) return;
  const title = reel.querySelector<HTMLElement>('#reel-title');
  const typed = reel.querySelector<HTMLElement>('#reel-type');
  let typer: ReturnType<typeof setInterval> | undefined;

  const show = (index: number): void => {
    frames.forEach((frame, i) => frame.classList.toggle('is-on', i === index));
    if (title) title.textContent = frames[index]!.dataset.name ?? '';
    const cmd = frames[index]!.dataset.cmd ?? '';
    clearInterval(typer);
    if (!typed) return;
    typed.textContent = '';
    let n = 0;
    typer = setInterval(() => {
      typed.textContent = cmd.slice(0, ++n);
      if (n >= cmd.length) clearInterval(typer);
    }, TYPE_AT);
  };

  const asked = new URLSearchParams(location.search).get('scene');
  const first = Math.max(
    0,
    frames.findIndex((frame) => frame.dataset.name === asked),
  );
  show(first);
  setInterval(() => {
    const current = frames.findIndex((frame) => frame.classList.contains('is-on'));
    show((current + 1) % frames.length);
  }, HOLD);
}
