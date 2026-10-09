// The braille layer for the wr documentation site.
//
// Everything is drawn with Braille patterns (U+2800..U+28FF), the same glyphs
// the reader uses: 2 columns x 4 rows of dots per cell. The background is a
// height field sampled per dot, the cursor is a blinking braille cell with a
// trail, clicks send ripples through the field and a burst of glyphs, and the
// footer progress bar fills cell by cell like the one in the program.
(() => {
  'use strict';

  const BASE = 0x2800;
  // Braille dot bits, [row][column] (same map as braille.go).
  const DOT = [[0x01, 0x08], [0x02, 0x10], [0x04, 0x20], [0x40, 0x80]];
  // Fill levels from low to full, exactly as in braille.go.
  const FILL = ['\u28C0', '\u28C4', '\u28E4', '\u28E6', '\u28F6', '\u28F7', '\u28FF']; // ⣀⣄⣤⣦⣶⣷⣿
  // Single-column fills, for icons.
  const ICONS = ['\u2801', '\u2803', '\u2807', '\u2847', '\u284F', '\u28FF']; // ⠁⠃⠇⡇⣇⣿
  const SPIN = ['\u280B', '\u2819', '\u2839', '\u2838', '\u283C', '\u2834', '\u2826', '\u2827', '\u2807', '\u280F']; // ⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏
  const WAVE_COLORS = ['#3b82d6', '#60a5fa', '#22d3ee', '#34d399', '#22d3ee', '#60a5fa'];
  const BLANK = '\u2800';

  const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
  const fine = matchMedia('(pointer: fine)').matches;

  // ---------------------------------------------------------------- field --
  // A full-screen animated braille field. Each dot samples a height field made
  // of traveling sine waves; dots over the threshold light up. The mouse is a
  // local bump and every click drops a ripple that decays.
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
      this.resize = this.resize.bind(this);
      this.loop = this.loop.bind(this);
      addEventListener('resize', this.resize);
      addEventListener('pointermove', (e) => {
        this.mouse.x = e.clientX;
        this.mouse.y = e.clientY;
      }, { passive: true });
      addEventListener('pointerdown', (e) => this.ripple(e.clientX, e.clientY), { passive: true });
      document.addEventListener('visibilitychange', () => {
        if (document.hidden) this.stop();
        else if (!reduce) this.start();
      });
      this.resize();
      if (reduce) this.render();
      else this.start();
    }

    start() {
      if (!this.raf) this.raf = requestAnimationFrame(this.loop);
    }

    stop() {
      cancelAnimationFrame(this.raf);
      this.raf = 0;
    }

    resize() {
      const w = innerWidth;
      const h = innerHeight;
      this.cols = Math.ceil(w / this.cellW);
      this.rows = Math.ceil(h / this.cellH);
      this.cv.width = Math.floor(w * this.dpr);
      this.cv.height = Math.floor(h * this.dpr);
      this.cv.style.width = w + 'px';
      this.cv.style.height = h + 'px';
      this.ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
      this.ctx.font = (this.cellH - 2) + 'px ui-monospace, SFMono-Regular, Menlo, Consolas, "DejaVu Sans Mono", monospace';
      this.ctx.textBaseline = 'top';
      if (reduce) this.render();
    }

    ripple(x, y) {
      if (this.ripples.length < 8) {
        this.ripples.push({ x: x / this.cellW * 2, y: y / this.cellH * 4, age: 0 });
      }
    }

    loop(ts) {
      const dt = Math.min((ts - this.last) / 1000 || 0, 0.05);
      this.last = ts;
      this.t += dt;
      for (const r of this.ripples) r.age += dt;
      if (this.ripples.length) this.ripples = this.ripples.filter((r) => r.age < 2.6);
      if (this.t - this.lastDraw > 1 / 30) {
        this.render();
        this.lastDraw = this.t;
      }
      this.raf = requestAnimationFrame(this.loop);
    }

    render() {
      const { ctx, cols, rows, cellW, cellH, t } = this;
      ctx.clearRect(0, 0, cols * cellW, rows * cellH);
      const W = cols * 2;
      const H = rows * 4;

      // Per-axis wave tables: the waves are split so each dot is a few
      // multiplies instead of a pile of Math.sin calls.
      const sx = new Float32Array(W), cx = new Float32Array(W);
      const s3x = new Float32Array(W), c3x = new Float32Array(W);
      for (let x = 0; x < W; x++) {
        const a = x * 0.16 + t * 1.1;
        const b = x * 0.11 + t * 0.5;
        sx[x] = Math.sin(a); cx[x] = Math.cos(a);
        s3x[x] = Math.sin(b); c3x[x] = Math.cos(b);
      }
      const sy = new Float32Array(H), s3y = new Float32Array(H), c3y = new Float32Array(H);
      for (let y = 0; y < H; y++) {
        sy[y] = Math.sin(y * 0.34 - t * 0.8);
        const b = y * 0.11;
        s3y[y] = Math.sin(b); c3y[y] = Math.cos(b);
      }

      const mx = this.mouse.x / cellW * 2;
      const my = this.mouse.y / cellH * 4;
      const ripples = this.ripples;

      let band = -1;
      for (let row = 0; row < rows; row++) {
        // The field fades with depth so the reading area stays calm.
        ctx.globalAlpha = Math.max(0.1, 1 - (row / rows) * 1.7);
        const y0 = row * 4;
        for (let col = 0; col < cols; col++) {
          const x0 = col * 2;
          let bits = 0;
          let sum = 0;
          for (let ly = 0; ly < 4; ly++) {
            const y = y0 + ly;
            const base = sy[y] + 0.4 * (s3x[x0] * c3y[y] + c3x[x0] * s3y[y]);
            for (let lx = 0; lx < 2; lx++) {
              const x = x0 + lx;
              let v = sx[x] + base;
              const dx = x - mx, dy = y - my;
              const md = dx * dx + dy * dy;
              if (md < 900) v += 1.6 * Math.exp(-md / 300);
              for (let i = 0; i < ripples.length; i++) {
                const r = ripples[i];
                const rx = x - r.x, ry = y - r.y;
                const d2 = rx * rx + ry * ry;
                if (d2 < 6400) {
                  const d = Math.sqrt(d2);
                  v += 1.5 * Math.exp(-r.age * 1.1) * Math.cos(d * 0.35 - r.age * 9) * Math.exp(-d * 0.02);
                }
              }
              sum += v;
              if (v > 0.72) bits |= DOT[ly][lx];
            }
          }
          if (!bits) continue;
          const avg = sum / 8;
          const b = avg > 1.15 ? 2 : avg > 0.9 ? 1 : 0;
          if (b !== band) {
            band = b;
            ctx.fillStyle = b === 2 ? '#7dd3fc' : b === 1 ? '#3b82d6' : '#1d3a63';
          }
          ctx.fillText(String.fromCharCode(BASE + bits), col * cellW, row * cellH);
        }
      }
    }
  }

  // --------------------------------------------------------------- cursor --
  // A braille cell that follows the pointer with a springy trail. It spins
  // while moving, blinks when idle, fills up over links and shows the link
  // target in a small label (like wr's link hints).
  class Cursor {
    constructor() {
      this.x = innerWidth / 2;
      this.y = innerHeight / 2;
      this.tx = this.x;
      this.ty = this.y;
      this.el = document.createElement('div');
      this.el.className = 'cursor';
      this.el.textContent = '\u283F';
      this.ghosts = [0, 1, 2].map((i) => {
        const g = document.createElement('div');
        g.className = 'cursor-ghost g' + i;
        document.body.append(g);
        return { el: g, x: this.x, y: this.y };
      });
      this.label = document.createElement('div');
      this.label.className = 'cursor-label';
      this.label.hidden = true;
      document.body.append(this.el, this.label);
      this.down = false;
      this.hover = null;
      this.frame = 0;
      this.t = 0;
      this.last = 0;

      addEventListener('pointermove', (e) => {
        this.tx = e.clientX;
        this.ty = e.clientY;
        const hit = e.target && e.target.closest ? e.target.closest('a[href], button, .copy') : null;
        this.setHover(hit);
      }, { passive: true });
      addEventListener('pointerdown', () => { this.down = true; });
      addEventListener('pointerup', () => { this.down = false; });
      document.documentElement.addEventListener('pointerleave', () => {
        this.el.style.opacity = '0';
        this.label.hidden = true;
      });
      document.documentElement.addEventListener('pointerenter', () => {
        this.el.style.opacity = '1';
      });

      this.loop = this.loop.bind(this);
      requestAnimationFrame(this.loop);
    }

    setHover(el) {
      this.hover = el;
      if (!el) { this.label.hidden = true; return; }
      let text = el.dataset.label;
      if (!text) {
        if (el.tagName === 'A') {
          try { text = new URL(el.href, location.href).hostname || el.textContent; }
          catch (_) { text = el.textContent; }
        } else {
          text = el.textContent;
        }
      }
      text = (text || '').trim().replace(/\s+/g, ' ');
      if (text.length > 44) text = text.slice(0, 43) + '\u2026';
      this.label.textContent = '\u283F ' + text;
      this.label.hidden = false;
    }

    loop(ts) {
      const dt = Math.min((ts - this.last) / 1000 || 0, 0.05);
      this.last = ts;
      this.t += dt;
      const k = 1 - Math.pow(0.0001, dt);
      this.x += (this.tx - this.x) * Math.min(1, k * 1.4);
      this.y += (this.ty - this.y) * Math.min(1, k * 1.4);
      const speed = Math.hypot(this.tx - this.x, this.ty - this.y);

      let ch;
      if (this.down) {
        ch = '\u28FF';
      } else if (this.hover) {
        ch = '\u28BF';
      } else if (speed > 1.5) {
        ch = SPIN[this.frame++ % SPIN.length];
      } else {
        ch = Math.floor(this.t * 1.8) % 2 ? '\u283F' : '\u280F';
      }
      this.el.textContent = ch;
      this.el.style.transform = 'translate3d(' + this.x + 'px,' + this.y + 'px,0)';
      this.label.style.transform = 'translate3d(' + (this.x + 16) + 'px,' + (this.y + 16) + 'px,0)';

      const lag = [0.10, 0.05, 0.025];
      const chars = ['\u2804', '\u2801', '\u2800'];
      this.ghosts.forEach((g, i) => {
        g.x += (this.x - g.x) * Math.min(1, k * lag[i]);
        g.y += (this.y - g.y) * Math.min(1, k * lag[i]);
        g.el.textContent = chars[i];
        g.el.style.transform = 'translate3d(' + g.x + 'px,' + g.y + 'px,0)';
      });
      requestAnimationFrame(this.loop);
    }
  }

  // -------------------------------------------------------------- effects --
  function burst(x, y) {
    if (reduce) return;
    for (let i = 0; i < 12; i++) {
      const s = document.createElement('span');
      s.className = 'spark';
      const set = Math.random() < 0.55 ? SPIN : ICONS;
      s.textContent = set[Math.floor(Math.random() * set.length)];
      s.style.left = x + 'px';
      s.style.top = y + 'px';
      const a = Math.random() * Math.PI * 2;
      const d = 28 + Math.random() * 76;
      const anim = s.animate([
        { transform: 'translate(-50%,-50%) scale(1)', opacity: 1 },
        { transform: 'translate(calc(-50% + ' + (Math.cos(a) * d) + 'px), calc(-50% + ' + (Math.sin(a) * d) + 'px)) scale(.35)', opacity: 0 },
      ], { duration: 480 + Math.random() * 420, easing: 'cubic-bezier(.2,.7,.3,1)' });
      anim.onfinish = () => s.remove();
      document.body.append(s);
    }
    const ring = document.createElement('span');
    ring.className = 'ring';
    ring.style.left = x + 'px';
    ring.style.top = y + 'px';
    const anim = ring.animate([
      { transform: 'translate(-50%,-50%) scale(.25)', opacity: 0.9 },
      { transform: 'translate(-50%,-50%) scale(1.7)', opacity: 0 },
    ], { duration: 620, easing: 'ease-out' });
    anim.onfinish = () => ring.remove();
    document.body.append(ring);
  }

  // ------------------------------------------------------------- braille --
  // Letters drawn as 5x7 dot maps, scaled 2x and packed into braille cells.
  const GLYPHS = {
    w: ['10001', '10001', '10101', '10101', '10101', '11011', '10001'],
    r: ['00000', '10000', '10000', '11110', '10001', '10001', '10001'],
  };

  function brailleArt(words) {
    const scale = 2;
    const gap = 2;
    const dotW = 5 * scale;
    const dotH = 7 * scale;
    const chars = [...words];
    const width = chars.length * dotW + (chars.length - 1) * gap;
    const grid = Array.from({ length: dotH }, () => new Array(width).fill(0));
    chars.forEach((ch, li) => {
      const rows = GLYPHS[ch];
      const xo = li * (dotW + gap);
      rows.forEach((row, y) => {
        for (let x = 0; x < row.length; x++) {
          if (row[x] !== '1') continue;
          for (let sy = 0; sy < scale; sy++) {
            for (let sx = 0; sx < scale; sx++) grid[y * scale + sy][xo + x * scale + sx] = 1;
          }
        }
      });
    });

    const cols = Math.ceil(width / 2);
    const rows = Math.ceil(dotH / 4);
    const out = [];
    for (let r = 0; r < rows; r++) {
      let line = '';
      for (let c = 0; c < cols; c++) {
        let bits = 0;
        for (let ly = 0; ly < 4; ly++) {
          const y = r * 4 + ly;
          if (y >= dotH) continue;
          for (let lx = 0; lx < 2; lx++) {
            const x = c * 2 + lx;
            if (x < width && grid[y][x]) bits |= DOT[ly][lx];
          }
        }
        line += String.fromCharCode(BASE + bits);
      }
      out.push(line);
    }
    return out;
  }

  function flash(pre, spans) {
    if (reduce) return;
    clearInterval(pre._flash);
    pre._flash = setInterval(() => {
      if (document.hidden) return;
      const s = spans[Math.floor(Math.random() * spans.length)];
      if (s.textContent === BLANK) return;
      s.classList.add('lit');
      setTimeout(() => s.classList.remove('lit'), 260);
    }, 900);
  }

  function renderLogo(pre) {
    const art = brailleArt('wr');
    const spans = [];
    pre.textContent = '';
    art.forEach((line, r) => {
      [...line].forEach((ch, c) => {
        const s = document.createElement('span');
        s.textContent = ch;
        s.style.animationDelay = (c * 45 + r * 12) + 'ms';
        pre.append(s);
        spans.push(s);
      });
      pre.append(document.createTextNode('\n'));
    });
    pre.classList.remove('run');
    void pre.offsetWidth;
    pre.classList.add('run');
    flash(pre, spans);
  }

  // The loading wave from braille.go, in a line of spans.
  function startWave(el, cells) {
    const spans = [];
    for (let i = 0; i < cells; i++) {
      const s = document.createElement('span');
      s.textContent = FILL[0];
      el.append(s);
      spans.push(s);
    }
    if (reduce) {
      spans.forEach((s, i) => { s.textContent = FILL[3 + (i % 4)]; });
      return;
    }
    let last = 0;
    const tick = (ts) => {
      if (ts - last > 33) {
        last = ts;
        const t = ts / 1000;
        for (let i = 0; i < cells; i++) {
          const level = (Math.sin(t * 7 - i * 0.5) + 1) / 2;
          spans[i].textContent = FILL[Math.round(level * (FILL.length - 1))];
          spans[i].style.color = WAVE_COLORS[(i + Math.floor(t * 9)) % WAVE_COLORS.length];
        }
      }
      requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  }

  // Footer progress, filled like progressBar() in braille.go.
  function startProgress(bar, pct) {
    let cells = 0;
    const spans = [];
    const build = () => {
      const n = Math.max(8, Math.min(30, Math.floor(innerWidth / 64)));
      if (n === cells) return;
      cells = n;
      bar.textContent = '';
      spans.length = 0;
      for (let i = 0; i < n; i++) {
        const s = document.createElement('span');
        s.textContent = FILL[0];
        bar.append(s);
        spans.push(s);
      }
      update();
    };
    const update = () => {
      const max = document.documentElement.scrollHeight - innerHeight;
      const p = max > 0 ? Math.min(100, Math.max(0, scrollY / max * 100)) : 0;
      const steps = FILL.length - 1;
      const units = Math.round(p * cells * steps / 100);
      for (let i = 0; i < cells; i++) {
        const u = Math.min(steps, Math.max(0, units - i * steps));
        spans[i].textContent = FILL[u];
        spans[i].className = u === 0 ? 'off' : '';
      }
      pct.textContent = Math.round(p) + '%';
    };
    addEventListener('resize', build);
    addEventListener('scroll', update, { passive: true });
    build();
  }

  // Copy buttons: spin, then confirm.
  function setupCopy() {
    document.querySelectorAll('[data-copy]').forEach((btn) => {
      btn.addEventListener('click', async (e) => {
        e.preventDefault();
        const label = btn.querySelector('.copy-label') || btn;
        const original = label.textContent;
        try { await navigator.clipboard.writeText(btn.dataset.copy); } catch (_) {}
        let i = 0;
        const spin = setInterval(() => { label.textContent = SPIN[i++ % SPIN.length] + ' copying'; }, 70);
        setTimeout(() => {
          clearInterval(spin);
          label.textContent = '\u283F copied';
          setTimeout(() => { label.textContent = original; }, 1500);
        }, 520);
      });
    });
  }

  // Card icons cycle through the fill levels while hovered.
  function setupCards() {
    document.querySelectorAll('.card .ico').forEach((ico) => {
      const base = ico.textContent;
      let iv = 0, i = 0;
      const card = ico.closest('.card');
      card.addEventListener('pointerenter', () => {
        if (reduce) return;
        clearInterval(iv);
        iv = setInterval(() => { ico.textContent = ICONS[i++ % ICONS.length]; }, 90);
      });
      card.addEventListener('pointerleave', () => {
        clearInterval(iv);
        ico.textContent = base;
      });
    });
  }

  // ------------------------------------------------------------------ go --
  function init() {
    const canvas = document.getElementById('field');
    if (canvas) new Field(canvas);
    if (fine && !reduce) {
      document.body.classList.add('has-cursor');
      new Cursor();
    }
    addEventListener('pointerdown', (e) => burst(e.clientX, e.clientY), { passive: true });

    const logo = document.getElementById('logo');
    if (logo) {
      renderLogo(logo);
      logo.addEventListener('click', () => renderLogo(logo));
    }
    const waveEl = document.getElementById('wave');
    if (waveEl) startWave(waveEl, Math.max(24, Math.min(72, Math.floor(innerWidth / 14))));

    const bar = document.getElementById('progress');
    const pct = document.getElementById('pct');
    if (bar && pct) startProgress(bar, pct);

    document.querySelectorAll('.rule').forEach((el) => { el.textContent = '\u28C0'.repeat(160); });

    setupCopy();
    setupCards();
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})();
