// The footer progress: the reading position drawn as a braille bar, filling
// cell by cell like the program's, with the percentage beside it.

const FILL = ['⣀', '⣄', '⣤', '⣦', '⣶', '⣷', '⣿'];
const BLANK = '⠀';

export function startProgress(): void {
  const bar = document.getElementById('progress');
  const pct = document.getElementById('pct');
  if (!bar || !pct) return;

  const paint = (): void => {
    const doc = document.documentElement;
    const max = doc.scrollHeight - innerHeight;
    const progress = max > 0 ? Math.min(1, Math.max(0, scrollY / max)) : 0;
    const cells = 12;
    const on = Math.round(progress * cells);
    let out = '';
    for (let i = 0; i < cells; i++) {
      if (i < on) out += FILL[6]!;
      else if (i === on) out += FILL[Math.min(6, Math.floor((progress * cells - on) * 6.99))] ?? BLANK;
      else out += BLANK;
    }
    bar.textContent = out || BLANK;
    pct.textContent = `${Math.round(progress * 100)}%`;
  };

  addEventListener('scroll', paint, { passive: true });
  addEventListener('resize', paint);
  paint();
}
