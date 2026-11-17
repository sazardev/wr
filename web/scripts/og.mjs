// The social card: public/assets/og.png, 1200x630, rasterised from an SVG with
// sharp. It is the same frame the site draws: terminal black, 16-color accents,
// a reading column and the reader's footer. Regenerate with `npm run og`.

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

const c = {
  bg: '#111315',
  panel: '#181b1e',
  line: '#24282c',
  line2: '#343b41',
  dim: '#747b76',
  fg2: '#a8adaa',
  fg: '#c5c8c6',
  accent: '#7aa6da',
  alt: '#70c0b1',
  green: '#b9ca4a',
  yellow: '#e7c547',
  magenta: '#c397d8',
  red: '#d54e53',
};

const mono = "ui-monospace, 'DejaVu Sans Mono', 'Liberation Mono', Menlo, monospace";
const text = (x, y, value, size, fill, weight = 400, extra = '') =>
  `<text x="${x}" y="${y}" font-family="${mono}" font-size="${size}" font-weight="${weight}" fill="${fill}" ${extra}>${value}</text>`;
const rule = (x, y, width, glyph, fill) =>
  text(x, y, glyph.repeat(Math.ceil(width / 10)), 18, fill, 400, `letter-spacing="0"`);

const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 1200 630">
  <rect width="1200" height="630" fill="${c.bg}"/>

  ${text(40, 44, '⣿', 22, c.accent, 700)}
  ${text(72, 44, 'wr', 22, c.fg, 700)}
  ${text(1128, 44, '⡇', 22, c.line2, 400)}

  ${rule(40, 66, 300, '⣀', c.line2)}
  ${rule(356, 66, 300, '⣀', c.accent)}
  ${rule(672, 66, 488, '⣀', c.alt)}

  ${text(64, 122, 'guide / start', 17, c.dim)}
  ${text(64, 164, '⣿ Building a Realtime Forum in Go', 29, c.accent, 700)}
  ${rule(64, 180, 980, '⣀', c.line2)}

  ${text(64, 222, 'Build a 100% CLI realtime forum in Go: TCP, TLS,', 19, c.fg2)}
  ${text(64, 250, 'deduplicated broadcast and resilient reconnects,', 19, c.fg2)}
  ${text(64, 278, 'zero dependencies.', 19, c.fg2)}

  <rect x="64" y="312" width="670" height="156" fill="none" stroke="${c.line2}"/>
  ${text(76, 338, '╭─ go', 17, c.dim)}
  ${text(76, 368, '│ func main() {', 18, c.fg)}
  ${text(76, 394, '│     srv := forum.New(cfg)', 18, c.fg)}
  ${text(76, 420, '│     log.Fatal(srv.ListenTLS())', 18, c.fg)}
  ${text(76, 446, '╰─', 17, c.dim)}

  ${text(760, 338, '⣿', 18, c.accent)}
  ${text(792, 338, 'Start page', 17, c.fg)}
  ${text(760, 368, '⡇', 18, c.accent)}
  ${text(792, 368, 'The guide', 17, c.fg)}
  ${text(760, 398, '⣀', 18, c.accent)}
  ${text(792, 398, 'Performance', 17, c.fg)}
  ${text(760, 428, '⠿', 18, c.accent)}
  ${text(792, 428, 'Architecture', 17, c.fg)}

  ${text(64, 520, '<tspan fill="' + c.accent + '">o</tspan> open  <tspan fill="' + c.accent + '">/</tspan> search  <tspan fill="' + c.accent + '">H</tspan> back  <tspan fill="' + c.accent + '">L</tspan> fwd  <tspan fill="' + c.accent + '">f</tspan> links  <tspan fill="' + c.accent + '">t</tspan> index  <tspan fill="' + c.accent + '">m</tspan> menu', 17, c.dim)}

  ${text(1128, 520, '⣷⣀⣀⣀⣀⣀  6%', 17, c.dim, 400, 'text-anchor="end"')}
  ${text(64, 580, 'wr v' + version + ' · MIT · github.com/sazardev/wr', 16, c.dim)}
  ${text(1128, 580, 'Linux · macOS · one static binary', 16, c.dim, 400, 'text-anchor="end"')}

  ${rule(40, 600, 300, '⣀', c.line2)}
  ${rule(356, 600, 300, '⣀', c.accent)}
  ${rule(672, 600, 488, '⣀', c.alt)}
</svg>`;

await sharp(Buffer.from(svg)).png({ compressionLevel: 9 }).toFile(out);
console.log(`og: wrote ${path.relative(root, out)} (v${version})`);
