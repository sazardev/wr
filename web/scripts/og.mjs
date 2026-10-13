// The social card: public/assets/og.png, 1200x630, rasterised from an SVG with
// sharp so the palette is exactly the site's blueprint. Social networks do not
// render SVG og:images, so this PNG is what every share shows.
//
// Regenerate with `npm run og` when the version or the copy changes.

import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import sharp from 'sharp';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '..');
const out = path.join(root, 'public/assets/og.png');

const version = (() => {
  try {
    const manifest = JSON.parse(readFileSync(path.join(root, '../.release-please-manifest.json'), 'utf8'));
    return String(manifest['.'] ?? 'dev').replace(/^v/, '');
  } catch {
    return 'dev';
  }
})();

const ink = {
  bg: '#050912',
  panel: '#0a1222',
  line: '#131f36',
  line2: '#22365b',
  dim: '#6f86ab',
  fg2: '#a3b5d3',
  fg: '#e8f0fd',
  accent: '#6cb4ff',
};

const mono = "ui-monospace, 'DejaVu Sans Mono', 'Liberation Mono', Menlo, monospace";
const sans = "'DejaVu Sans', 'Liberation Sans', Inter, system-ui, sans-serif";

const tick = (x, y) =>
  `<path d="M ${x - 7} ${y} H ${x + 7} M ${x} ${y - 7} V ${y + 7}" stroke="${ink.line2}" stroke-width="1"/>`;

const line = (x, y, text, size, fill, family, weight = 400, extra = '') =>
  `<text x="${x}" y="${y}" font-family="${family}" font-size="${size}" font-weight="${weight}" fill="${fill}" ${extra}>${text}</text>`;

const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 1200 630">
  <rect width="1200" height="630" fill="${ink.bg}"/>
  <rect x="28.5" y="28.5" width="1143" height="573" fill="none" stroke="${ink.line2}" stroke-width="1"/>
  ${tick(28, 28)}${tick(1172, 28)}${tick(28, 602)}${tick(1172, 602)}

  ${line(64, 86, 'wr<span fill="' + ink.accent + '">▍</span>', 34, ink.fg, mono, 700)}
  ${line(1136, 84, `v${version} · MIT`, 20, ink.dim, mono, 400, 'text-anchor="end"')}

  ${line(64, 242, 'The web,', 72, ink.fg, sans, 700, 'letter-spacing="-2"')}
  ${line(64, 318, 'without the noise', 72, ink.fg, sans, 700, 'letter-spacing="-2"')}

  ${line(64, 386, 'Reader mode for your terminal:', 23, ink.fg2, sans)}
  ${line(64, 418, 'formatted Markdown, real code highlighting,', 23, ink.fg2, sans)}
  ${line(64, 450, 'links, history, bookmarks and search.', 23, ink.fg2, sans)}

  ${line(64, 560, 'github.com/sazardev/wr', 21, ink.dim, mono)}
  ${line(1136, 560, 'Linux · macOS · Go', 21, ink.dim, mono, 400, 'text-anchor="end"')}

  <g>
    <rect x="680" y="112" width="456" height="404" fill="${ink.panel}" stroke="${ink.line2}"/>
    <rect x="680" y="112" width="456" height="34" fill="${ink.bg}" stroke="${ink.line2}"/>
    ${line(700, 134, 'wr — article', 14, ink.dim, mono)}
    ${line(1116, 134, '100 × 30', 14, ink.dim, mono, 400, 'text-anchor="end"')}

    ${line(708, 190, '<tspan fill="' + ink.accent + '">▍</tspan>Building a Realtime Forum in Go', 17, ink.fg, sans, 600)}
    <rect x="708" y="204" width="400" height="2" fill="${ink.accent}" opacity="0.85"/>

    ${line(708, 240, 'Build a 100% CLI realtime forum in Go: TCP,', 14, ink.fg2, sans)}
    ${line(708, 262, 'TLS, deduplicated broadcast and resilient', 14, ink.fg2, sans)}
    ${line(708, 284, 'reconnects, zero dependencies.', 14, ink.fg2, sans)}

    <rect x="708" y="306" width="400" height="128" fill="${ink.bg}" stroke="${ink.line2}"/>
    ${line(724, 334, '// main.go', 13, ink.dim, mono)}
    ${line(724, 358, '<tspan fill="' + ink.accent + '">func</tspan> main() {', 13, ink.fg2, mono)}
    ${line(724, 382, '    srv := forum.New(cfg)', 13, ink.fg2, mono)}
    ${line(724, 406, '    log.Fatal(srv.ListenTLS())', 13, ink.fg2, mono)}
    ${line(724, 430, '}', 13, ink.fg2, mono)}

    <rect x="708" y="452" width="400" height="1" fill="${ink.line2}"/>
    ${line(708, 480, '<tspan fill="' + ink.accent + '">o</tspan> open   <tspan fill="' + ink.accent + '">/</tspan> search   <tspan fill="' + ink.accent + '">H</tspan> back   <tspan fill="' + ink.accent + '">f</tspan> links   <tspan fill="' + ink.accent + '">m</tspan> menu', 13, ink.dim, mono)}
  </g>
</svg>`;

await sharp(Buffer.from(svg)).png({ compressionLevel: 9 }).toFile(out);
console.log(`og: wrote ${path.relative(root, out)} (v${version})`);
