import { themeColor } from './themes';

// The braille field: a full-screen height map drawn with the same glyphs the
// reader uses, in the accent color of the current theme. Flat by rule: one
// color, opacity carries the height. It follows the pointer and ripples on
// click, and it stops while the tab is hidden.

const BASE = 0x2800;
const DOT = [
  [0x01, 0x08],
  [0x02, 0x10],
  [0x04, 0x20],
  [0x40, 0x80],
];

interface Ripple {
  x: number;
  y: number;
  age: number;
}

export class BrailleField {
  private ctx!: CanvasRenderingContext2D;
  private readonly ripples: Ripple[] = [];
  private readonly mouse = { x: -1e5, y: -1e5 };
  private readonly onTheme = (): void => this.redraw();

  private cols = 0;
  private rows = 0;
  private t = 0;
  private last = 0;
  private raf = 0;

  constructor(private readonly canvas: HTMLCanvasElement) {
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    this.ctx = ctx;
    this.dpr = Math.min(devicePixelRatio || 1, 1.5);

    addEventListener('resize', () => this.resize());
    addEventListener('pointermove', (event) => {
      this.mouse.x = event.clientX;
      this.mouse.y = event.clientY;
    }, { passive: true });
    addEventListener('pointerdown', (event) => this.ripple(event.clientX, event.clientY), { passive: true });
    document.addEventListener('visibilitychange', () => {
      if (document.hidden) this.stop();
      else this.start();
    });
    document.addEventListener('wr-theme', this.onTheme);

    this.resize();
    this.start();
  }

  private readonly cellW = 11;
  private readonly cellH = 20;
  private readonly dpr!: number;

  private get color(): string {
    return themeColor(document.documentElement.dataset.theme ?? 'wr');
  }

  private start(): void {
    if (this.raf) return;
    this.last = performance.now();
    this.raf = requestAnimationFrame(this.loop);
  }

  private stop(): void {
    if (!this.raf) return;
    cancelAnimationFrame(this.raf);
    this.raf = 0;
  }

  private resize(): void {
    this.cols = Math.ceil(innerWidth / this.cellW) + 1;
    this.rows = Math.ceil(innerHeight / this.cellH) + 1;
    this.canvas.width = this.cols * this.cellW * this.dpr;
    this.canvas.height = this.rows * this.cellH * this.dpr;
    this.canvas.style.width = `${this.cols * this.cellW}px`;
    this.canvas.style.height = `${this.rows * this.cellH}px`;
    this.ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
    this.ctx.font = `${this.cellH}px ui-monospace, monospace`;
    this.ctx.textBaseline = 'top';
    this.redraw();
  }

  private ripple(x: number, y: number): void {
    this.ripples.push({ x, y, age: 0 });
    if (this.ripples.length > 12) this.ripples.shift();
    this.start();
  }

  private height(x: number, y: number, t: number): number {
    let h =
      Math.sin(x * 0.16 + t * 0.9) * 0.5 +
      Math.sin(y * 0.21 - t * 0.7) * 0.5 +
      Math.sin((x + y) * 0.09 + t * 1.4) * 0.5;
    const dx = x * this.cellW - this.mouse.x;
    const dy = y * this.cellH - this.mouse.y;
    const d = Math.sqrt(dx * dx + dy * dy);
    if (d < 90) h += (1 - d / 90) * 2.2;
    for (const r of this.ripples) {
      const dist = Math.hypot(x * this.cellW - r.x, y * this.cellH - r.y);
      const w = Math.exp(-Math.pow(dist - r.age * 260, 2) / 6000) * (1 - r.age / 1.6);
      if (w > 0) h += w * 2.4;
    }
    return h;
  }

  private loop = (now: number): void => {
    const dt = Math.min((now - this.last) / 1000, 0.05);
    this.last = now;
    this.t += dt;
    for (const r of this.ripples) r.age += dt;
    this.ripples.splice(0, this.ripples.length, ...this.ripples.filter((r) => r.age < 1.6));
    this.redraw();
    this.raf = requestAnimationFrame(this.loop);
  };

  private redraw(): void {
    if (!this.raf && this.t === 0) return; // not started yet
    const { ctx, cols, rows } = this;
    const t = this.t;
    ctx.clearRect(0, 0, cols * this.cellW, rows * this.cellH);
    for (let y = 0; y < rows; y++) {
      for (let x = 0; x < cols; x++) {
        const h = this.height(x, y, t);
        if (h < -0.8) continue;
        let bits = 0;
        for (let r = 0; r < 4; r++) {
          for (let c = 0; c < 2; c++) {
            const o = Math.sin((x + c * 0.7) * 0.4 + t * 2 + (y + r * 0.5) * 0.3) * 0.6 + h;
            if (o > 1.5) bits |= DOT[r]![c]!;
          }
        }
        if (!bits) continue;
        ctx.globalAlpha = Math.min(0.04 + Math.max(0, h + 0.8) * 0.09, 0.2);
        ctx.fillStyle = this.color;
        ctx.fillText(String.fromCharCode(BASE + bits), x * this.cellW, y * this.cellH);
      }
    }
    ctx.globalAlpha = 1;
  }
}

/** One field per canvas, for the whole life of the document. */
const fields = new WeakMap<HTMLCanvasElement, BrailleField>();

export function startField(): void {
  const canvas = document.getElementById('field') as HTMLCanvasElement | null;
  if (!canvas || fields.has(canvas)) return;
  const field = new BrailleField(canvas);
  if (field) fields.set(canvas, field);
}
