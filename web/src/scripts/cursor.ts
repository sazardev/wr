// The custom cursor: a blinking braille cell with a trail of ghosts, only on
// pointers that can actually hover (a touch screen never sees it). Over a link
// or a button the trail shows the "open" glyph.

const DOT = '⠹';
const GHOST = ['⠇', '⠃', '⠁'];
const OPEN = '⠇';

export function startCursor(): void {
  if (!matchMedia('(pointer: fine)').matches) return;
  if (document.body.dataset.cursorOn === '1') return;
  document.body.dataset.cursorOn = '1';

  const dot = document.createElement('div');
  dot.className = 'cursor blink';
  dot.textContent = DOT;
  document.body.appendChild(dot);
  document.body.classList.add('has-cursor');

  const ghosts = GHOST.map((glyph) => {
    const el = document.createElement('div');
    el.className = `cursor-ghost g${GHOST.indexOf(glyph)}`;
    el.textContent = glyph;
    document.body.appendChild(el);
    return { el, x: innerWidth / 2, y: innerHeight / 2 };
  });

  addEventListener(
    'pointermove',
    (event) => {
      const { clientX: x, clientY: y } = event;
      dot.style.transform = `translate(${x}px, ${y}px)`;
      const interactive = Boolean((event.target as Element | null)?.closest('a,button'));
      for (const ghost of ghosts) {
        ghost.x += (x - ghost.x) * 0.16;
        ghost.y += (y - ghost.y) * 0.16;
        ghost.el.style.transform = `translate(${ghost.x}px, ${ghost.y}px)`;
        ghost.el.textContent = interactive ? OPEN : GHOST[ghosts.indexOf(ghost)]!;
      }
    },
    { passive: true },
  );
}
