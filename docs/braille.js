// The braille layer for the wr documentation site (design system DS-1.0).
//
// It owns four things, all flat and all drawn with the same glyphs the reader
// uses:
//
//   1. themes: six palettes (bg, surface, fg, dim, accent, accent2) applied to
//      <html data-theme>, chosen by the visitor and remembered, following the
//      system preference when there is no choice;
//   2. the field: a full-screen braille height map in the accent color, with a
//      cursor bump and click ripples;
//   3. the braille rules: ⣀ fills that match the width of their section, and the
//      footer progress bar, which fills cell by cell like the program's;
//   4. the custom cursor: a blinking braille cell with a trail;
//   5. the reel: the captured scenes cycling in the hero, with the command
//      typed under them.
//
// No gradient, no shadow, no glow: opacity over the accent color only.
// Motion is always on (DS-1.1: the reduced-motion gate was removed).
(() => {
  'use strict';

  const BASE = 0x2800;
  const DOT = [[0x01, 0x08], [0x02, 0x10], [0x04, 0x20], [0x40, 0x80]]; // [row][col]
  const FILL = ['\u28C0', '\u28C4', '\u28E4', '\u28E6', '\u28F6', '\u28F7', '\u28FF']; // ⣀⣄⣤⣦⣶⣷⣿
  const SPIN = ['\u280B', '\u2819', '\u2839', '\u2838', '\u283C', '\u2834', '\u2826', '\u2827', '\u2807', '\u280F']; // ⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏
  const BLANK = '\u2800';

  const fine = matchMedia('(pointer: fine)').matches;

  // ----------------------------------------------------------------- themes --
  // Six palettes, same shape as the tokens in style.css. The chips shown in the
  // header are built from here, so a theme is added in both places only.
  const THEMES = [
    { id: 'wr', keys: 'dark', colors: ['#07090d', '#0d1117', '#c9d3e3', '#5f7089', '#4da3ff'] },
    { id: 'gruvbox', keys: 'dark', colors: ['#282828', '#32302f', '#ebdbb2', '#928374', '#fabd2f'] },
    { id: 'nord', keys: 'dark', colors: ['#2e3440', '#343b49', '#d8dee9', '#616e88', '#88c0d0'] },
    { id: 'catppuccin', keys: 'dark', colors: ['#1e1e2e', '#181825', '#cdd6f4', '#6c7086', '#cba6f7'] },
    { id: 'solarized', keys: 'dark', colors: ['#002b36', '#073642', '#93a1a1', '#586e75', '#268bd2'] },
    { id: 'matrix', keys: 'dark', colors: ['#050805', '#0a120a', '#b6f3c4', '#4e8a5b', '#2ee66f'] },
    { id: 'paper', keys: 'light', colors: ['#f2efe9', '#e9e5db', '#20242a', '#6b7178', '#0b5cad'] },
  ];
  const STORE = 'wr-docs-theme';

  const token = (name) => getComputedStyle(document.documentElement).getPropertyValue(name).trim();

  function setTheme(id, persist = true) {
    const theme = THEMES.find((t) => t.id === id) || THEMES[0];
    document.documentElement.dataset.theme = theme.id;
    if (persist) {
      try { localStorage.setItem(STORE, theme.id); } catch {}
    }
    document.querySelectorAll('.themes button').forEach((b) => {
      b.setAttribute('aria-pressed', String(b.dataset.theme === theme.id));
    });
    document.dispatchEvent(new CustomEvent('theme', { detail: theme }));
  }

  function buildThemeBar() {
    const bar = document.getElementById('themes');
    if (!bar) return;
    for (const t of THEMES) {
      const b = document.createElement('button');
      b.dataset.theme = t.id;
      b.title = `${t.id} (${t.keys})`;
      b.setAttribute('aria-label', `theme: ${t.id}`);
      b.setAttribute('aria-pressed', 'false');
      // chips: bg, surface, fg and accent of that palette, nothing else
      for (const c of [t.colors[0], t.colors[1], t.colors[2], t.colors[4]]) {
        const i = document.createElement('i');
        i.style.background = c;
        b.appendChild(i);
      }
      b.addEventListener('click', () => setTheme(t.id));
      bar.appendChild(b);
    }
    const swatches = document.getElementById('swatches');
    if (swatches) {
      for (const t of THEMES) {
        const b = document.createElement('button');
        b.className = 'swatch';
        b.dataset.theme = t.id;
        b.setAttribute('aria-pressed', 'false');
        const chips = document.createElement('span');
        chips.className = 'chips';
        for (const c of [t.colors[0], t.colors[1], t.colors[2], t.colors[4]]) {
          const i = document.createElement('i');
          i.style.background = c;
          chips.appendChild(i);
        }
        const name = document.createElement('span');
        name.className = 'name';
        name.textContent = t.id;
        const keys = document.createElement('span');
        keys.className = 'keys';
        keys.textContent = t.keys;
        b.append(chips, name, keys);
        b.addEventListener('click', () => setTheme(t.id));
        swatches.appendChild(b);
      }
    }
  }

  // ------------------------------------------------------------------ field --
  class Field {
    constructor(canvas) {
      this.cv = canvas;
      this.ctx = canvas.getContext('2d');
      this.cellW = 11;
      this.cellH = 20;
      this.ripples = [];
      this.mouse = { x: -1e5, y: -1e5 };
      this.t = 0;
      this.last = 0;
      this.lastDraw = 0;
      this.raf = 0;
      this.dpr = Math.min(devicePixelRatio || 1, 1.5);
      this.color = token('--accent');
      this.resize = this.resize.bind(this);
      this.loop = this.loop.bind(this);
      document.addEventListener('theme', () => { this.color = token('--accent'); this.draw(0); });
      addEventListener('resize', this.resize);
      addEventListener('pointermove', (e) => { this.mouse.x = e.clientX; this.mouse.y = e.clientY; }, { passive: true });
      addEventListener('pointerdown', (e) => this.ripple(e.clientX, e.clientY), { passive: true });
      document.addEventListener('visibilitychange', () => {
        if (document.hidden) { cancelAnimationFrame(this.raf); this.raf = 0; }
        else this.start();
      });
      this.resize();
      this.start();
    }

    start() { if (!this.raf) { this.last = performance.now(); this.raf = requestAnimationFrame(this.loop); } }

    resize() {
      this.cols = Math.ceil(innerWidth / this.cellW) + 1;
      this.rows = Math.ceil(innerHeight / this.cellH) + 1;
      this.cv.width = this.cols * this.cellW * this.dpr;
      this.cv.height = this.rows * this.cellH * this.dpr;
      this.cv.style.width = this.cols * this.cellW + 'px';
      this.cv.style.height = this.rows * this.cellH + 'px';
      this.ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
      this.ctx.font = `${this.cellH}px ui-monospace, monospace`;
      this.ctx.textBaseline = 'top';
      this.draw(0);
    }

    height(x, y, t) {
      const h = Math.sin(x * 0.16 + t * 0.9) * 0.5 + Math.sin(y * 0.21 - t * 0.7) * 0.5 + Math.sin((x + y) * 0.09 + t * 1.4) * 0.5;
      const dx = x * this.cellW - this.mouse.x, dy = y * this.cellH - this.mouse.y;
      const d = Math.sqrt(dx * dx + dy * dy);
      if (d < 90) h += (1 - d / 90) * 2.2;
      for (const r of this.ripples) {
        const dist = Math.hypot(x * this.cellW - r.x, y * this.cellH - r.y);
        const w = Math.exp(-Math.pow(dist - r.age * 260, 2) / 6000) * (1 - r.age / 1.6);
        if (w > 0) h += w * 2.4;
      }
      return h;
    }

    ripple(x, y) {
      this.ripples.push({ x, y, age: 0 });
      if (this.ripples.length > 12) this.ripples.shift();
      this.start();
    }

    loop(now) {
      const dt = Math.min((now - this.last) / 1000, 0.05);
      this.last = now;
      this.t += dt;
      for (const r of this.ripples) r.age += dt;
      this.ripples = this.ripples.filter((r) => r.age < 1.6);
      this.draw(this.t);
      this.raf = requestAnimationFrame(this.loop);
    }

    draw(t) {
      const { ctx, cols, rows } = this;
      ctx.clearRect(0, 0, cols * this.cellW, rows * this.cellH);
      // flat: one color, opacity carries the height
      for (let y = 0; y < rows; y++) {
        for (let x = 0; x < cols; x++) {
          const h = this.height(x, y, t);
          if (h < -0.8) continue;
          const bits = [];
          for (let r = 0; r < 4; r++) {
            for (let c = 0; c < 2; c++) {
              const o = Math.sin((x + c * 0.7) * 0.4 + t * 2 + (y + r * 0.5) * 0.3) * 0.6 + h;
              if (o > 1.5) bits.push(DOT[r][c]);
            }
          }
          if (!bits.length) continue;
          const ch = String.fromCharCode(BASE + bits.reduce((a, b) => a | b, 0));
          const a = Math.min(0.04 + Math.max(0, h + 0.8) * 0.09, 0.2);
          ctx.globalAlpha = a;
          ctx.fillStyle = this.color;
          ctx.fillText(ch, x * this.cellW, y * this.cellH);
        }
      }
      ctx.globalAlpha = 1;
    }
  }

  // -------------------------------------------------------------- rules/fill --
  // A ⣀ fill that always matches the width of its row, like the app's rule.
  function measure(el) {
    const probe = document.createElement('span');
    probe.style.cssText = 'position:absolute;visibility:hidden;white-space:pre;font:inherit';
    probe.textContent = '\u28C0';
    el.appendChild(probe);
    const w = probe.getBoundingClientRect().width || 8;
    probe.remove();
    return Math.max(4, Math.floor(el.getBoundingClientRect().width / w));
  }

  function fillRules() {
    for (const el of document.querySelectorAll('.rule-b[data-fill]')) {
      const n = parseInt(el.dataset.fill, 10) || 0;
      const want = Math.max(1, n - 2);
      el.textContent = '\u28C0'.repeat(Math.min(want, measure(el)));
    }
  }

  // ------------------------------------------------------------- progress --
  function startProgress(bar, pct) {
    const doc = document.documentElement;
    const paint = () => {
      const max = doc.scrollHeight - innerHeight;
      const p = max > 0 ? Math.min(1, Math.max(0, scrollY / max)) : 0;
      const cells = 12;
      const on = Math.round(p * cells);
      let out = '';
      for (let i = 0; i < cells; i++) {
        if (i < on) out += FILL[6];
        else if (i === on) out += FILL[Math.min(6, Math.floor((p * cells - on) * 6.99))] || BLANK;
        else out += BLANK;
      }
      bar.textContent = out || BLANK;
      pct.textContent = Math.round(p * 100) + '%';
    };
    addEventListener('scroll', paint, { passive: true });
    addEventListener('resize', paint);
    paint();
  }

  // ---------------------------------------------------------------- cursor --
  function startCursor() {
    if (!fine) return;
    const cur = document.createElement('div');
    cur.className = 'cursor blink';
    cur.textContent = '\u2839'; // ⠹
    document.body.appendChild(cur);
    document.body.classList.add('has-cursor');
    const ghosts = [];
    for (let i = 1; i <= 3; i++) {
      const g = document.createElement('div');
      g.className = 'cursor-ghost g' + (i - 1);
      g.textContent = ['\u2807', '\u2803', '\u2801'][i - 1]; // ⠇⠃⠁
      document.body.appendChild(g);
      ghosts.push({ el: g, x: innerWidth / 2, y: innerHeight / 2 });
    }
    let x = innerWidth / 2, y = innerHeight / 2;
    addEventListener('pointermove', (e) => {
      x = e.clientX; y = e.clientY;
      cur.style.transform = `translate(${x}px, ${y}px)`;
      const label = (e.target.closest('a,button') || {}).textContent || '';
      for (const g of ghosts) {
        g.x += (x - g.x) * 0.16; g.y += (y - g.y) * 0.16;
        g.el.style.transform = `translate(${g.x}px, ${g.y}px)`;
        g.el.textContent = label && label.trim() ? '\u2807' : ['\u2807', '\u2803', '\u2801'][ghosts.indexOf(g)] || '\u2801';
      }
    }, { passive: true });
  }

  // ------------------------------------------------------------------ spin --
  function startSpin() {
    const dots = document.querySelectorAll('[data-spin]');
    if (!dots.length) return;
    let i = 0;
    setInterval(() => {
      i = (i + 1) % SPIN.length;
      dots.forEach((d) => { d.textContent = SPIN[i]; });
    }, 110);
  }

  // ------------------------------------------------------------------ reel --
  // The hero shows the real captured scenes, one at a time, with the command
  // that produced it typed underneath. Data lives in the HTML (data-name,
  // data-cmd); this only drives it.
  function startReel() {
    const reel = document.getElementById('reel');
    if (!reel) return;
    const frames = [...reel.querySelectorAll('.reel-frame')];
    const title = reel.querySelector('#reel-title');
    const type = reel.querySelector('#reel-type');
    if (!frames.length) return;
    let i = 0, typer = 0;
    const show = (n) => {
      frames.forEach((f, k) => f.classList.toggle('is-on', k === n));
      if (title) title.textContent = frames[n].dataset.name || '';
      const cmd = frames[n].dataset.cmd || '';
      clearInterval(typer);
      if (type) {
        type.textContent = '';
        let j = 0;
        typer = setInterval(() => {
          type.textContent = cmd.slice(0, ++j);
          if (j >= cmd.length) clearInterval(typer);
        }, 34);
      }
    };
    show(0);
    setInterval(() => { i = (i + 1) % frames.length; show(i); }, 4600);
  }

  // --------------------------------------------------------------- bootstrap --
  buildThemeBar();
  let stored = null;
  try { stored = localStorage.getItem(STORE); } catch {}
  setTheme(stored || (matchMedia('(prefers-color-scheme: light)').matches ? 'paper' : 'wr'), false);
  fillRules();
  addEventListener('resize', fillRules);
  if (document.fonts && document.fonts.ready) document.fonts.ready.then(fillRules);
  new Field(document.getElementById('field'));
  startCursor();
  startSpin();
  startReel();
  const bar = document.getElementById('progress'), pct = document.getElementById('pct');
  if (bar && pct) startProgress(bar, pct);
  // exposed for the console and for tests
  window.wrDocs = { THEMES, setTheme, fillRules };
})();
