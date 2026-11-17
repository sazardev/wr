// The web edition of wr's TUI: one fixed terminal frame, the same keys, the
// same braille chrome and the same panels. The server renders the content and
// the links; this script only adds the interaction the reader has.
//
// It is deliberately one file: the frame is replaced on every Astro view
// transition, so every function looks its elements up again instead of keeping
// stale nodes. Listeners live on `document`, which survives the swap.

import { navigate as astroNavigate } from 'astro:transitions/client';
import { searchEntries, tokenize, type SearchEntry } from '../lib/search';
import { progressBar, progressCells, readPercent, scrollbarChars, PLAIN_TRACK, TRACK } from '../lib/tui';
import { withBase } from '../lib/urls';

type Mode = 'read' | 'menu' | 'toc' | 'help' | 'settings' | 'omni' | 'find' | 'hints';

interface Settings {
  accent: string;
  width: 'narrow' | 'normal' | 'wide' | 'full';
  spacing: '0' | '1' | '2';
  headingRules: boolean;
  codeFrames: boolean;
  braille: boolean;
  center: boolean;
  images: boolean;
  videos: boolean;
  imageFit: 'contain' | 'cover';
  imageWidth: 'full' | 'wide' | 'half';
  videoAutoplay: boolean;
  videoLoop: boolean;
  videoMuted: boolean;
  videoControls: boolean;
}

const STORAGE = 'wr-web-settings';
const ACCENTS = [
  'blue', 'cyan', 'green', 'magenta', 'yellow', 'red', 'gray', 'white',
  'orange', 'violet', 'teal', 'pink', 'lime', 'slate',
] as const;
const DEFAULTS: Settings = {
  accent: 'blue',
  width: 'normal',
  spacing: '1',
  headingRules: true,
  codeFrames: true,
  braille: true,
  center: true,
  images: false,
  videos: false,
  imageFit: 'contain',
  imageWidth: 'full',
  videoAutoplay: false,
  videoLoop: false,
  videoMuted: true,
  videoControls: true,
};
const HINT_ALPHABET = 'asdfghjkl';
const INSTALL = 'curl -fsSL https://raw.githubusercontent.com/sazardev/wr/main/install.sh | sh';

const state: {
  mode: Mode;
  findQuery: string;
  findMarks: HTMLElement[];
  findIndex: number;
  menuIndex: number;
  tocIndex: number;
  settingsIndex: number;
  omniIndex: number;
  hints: { label: string; href: string }[];
  hintInput: string;
  toastTimer: number;
  settings: Settings;
  filterOpen: boolean;
  filter: string;
  lastPath: string;
  lastScroll: number;
} = {
  mode: 'read',
  findQuery: '',
  findMarks: [],
  findIndex: -1,
  menuIndex: 0,
  tocIndex: 0,
  settingsIndex: 0,
  omniIndex: 0,
  hints: [],
  hintInput: '',
  toastTimer: 0,
  settings: { ...DEFAULTS },
  filterOpen: false,
  filter: '',
  lastPath: '',
  lastScroll: 0,
};

let started = false;
let indexPromise: Promise<SearchEntry[]> | null = null;

// ---------------------------------------------------------------- helpers

function el<T extends Element>(sel: string, root: ParentNode = document): T | null {
  return root.querySelector<T>(sel);
}
function els<T extends Element>(sel: string, root: ParentNode = document): T[] {
  return Array.from(root.querySelectorAll<T>(sel));
}
function bodyEl(): HTMLElement | null {
  return el<HTMLElement>('[data-wr-body]');
}
function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]!);
}
function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}
function clamp(n: number, lo: number, hi: number): number {
  return Math.min(Math.max(n, lo), hi);
}
function lineHeight(node: Element | null): number {
  if (!node) return 21.7;
  const value = parseFloat(getComputedStyle(node).lineHeight);
  return Number.isFinite(value) ? value : 21.7;
}
function scrollByLines(n: number): void {
  const body = bodyEl();
  if (!body) return;
  body.scrollBy({ top: n * lineHeight(body), behavior: 'auto' });
}
function scrollByPage(dir: 1 | -1): void {
  const body = bodyEl();
  if (!body) return;
  body.scrollBy({ top: dir * body.clientHeight * 0.9, behavior: 'auto' });
}
function scrollTop(): void {
  bodyEl()?.scrollTo({ top: 0, behavior: 'auto' });
}
function scrollBottom(): void {
  const body = bodyEl();
  if (body) body.scrollTo({ top: body.scrollHeight, behavior: 'auto' });
}

// ---------------------------------------------------------------- settings

function loadSettings(): void {
  try {
    const raw = localStorage.getItem(STORAGE);
    if (!raw) return;
    const parsed = JSON.parse(raw) as Partial<Settings>;
    state.settings = { ...DEFAULTS, ...parsed };
  } catch {
    state.settings = { ...DEFAULTS };
  }
}
function applySettings(): void {
  const s = state.settings;
  const root = document.documentElement;
  root.dataset.accent = s.accent;
  root.dataset.width = s.width;
  root.dataset.spacing = s.spacing;
  root.dataset.headingRules = s.headingRules ? 'on' : 'off';
  root.dataset.codeFrames = s.codeFrames ? 'on' : 'off';
  root.dataset.braille = s.braille ? 'on' : 'off';
  root.dataset.center = s.center ? 'on' : 'off';
  root.dataset.images = s.images ? 'on' : 'off';
  root.dataset.videos = s.videos ? 'on' : 'off';
  root.dataset.mediaFit = s.imageFit;
  root.dataset.mediaWidth = s.imageWidth;
  root.dataset.videoAutoplay = s.videoAutoplay ? 'on' : 'off';
  root.dataset.videoLoop = s.videoLoop ? 'on' : 'off';
  root.dataset.videoMuted = s.videoMuted ? 'on' : 'off';
  root.dataset.videoControls = s.videoControls ? 'on' : 'off';
  try {
    localStorage.setItem(STORAGE, JSON.stringify(s));
  } catch {
    /* private mode: the settings still apply for this page */
  }
  updateChrome();
  scheduleMediaRefresh();
}


// ---------------------------------------------------------------- media

const MEDIA_CACHE = 'wr-media-v1';
const MEDIA_MANIFEST = 'wr-web-media-cache';
const MEDIA_MAX_IMAGE = 8 << 20;

let mediaObserver: IntersectionObserver | null = null;
let mediaRefreshTimer = 0;
const mediaObjectUrls = new Map<HTMLElement, string[]>();

function humanBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

function mediaCacheManifest(): Record<string, number> {
  try {
    return JSON.parse(localStorage.getItem(MEDIA_MANIFEST) || '{}') as Record<string, number>;
  } catch {
    return {};
  }
}

function mediaCacheSummary(): { count: number; bytes: number } {
  const values = Object.values(mediaCacheManifest());
  return { count: values.length, bytes: values.reduce((sum, n) => sum + (Number(n) || 0), 0) };
}

function recordMedia(url: string, size: number): void {
  const manifest = mediaCacheManifest();
  manifest[url] = size;
  try {
    localStorage.setItem(MEDIA_MANIFEST, JSON.stringify(manifest));
  } catch {
    /* private mode */
  }
  updateMediaCacheRow();
}

function updateMediaCacheRow(): void {
  const node = el<HTMLElement>('[data-media-cache-value]');
  if (!node) return;
  const { count, bytes } = mediaCacheSummary();
  node.textContent = count ? `${count} images · ${humanBytes(bytes)}` : 'empty';
}

function scheduleMediaRefresh(): void {
  if (mediaRefreshTimer) return;
  mediaRefreshTimer = window.setTimeout(() => {
    mediaRefreshTimer = 0;
    refreshMedia();
  }, 40);
}

function mediaKind(node: HTMLElement): 'image' | 'video' | '' {
  const kind = node.dataset.media;
  return kind === 'image' || kind === 'video' ? kind : '';
}

function mediaAllowed(node: HTMLElement): boolean {
  const kind = mediaKind(node);
  return kind === 'image' ? state.settings.images : kind === 'video' ? state.settings.videos : false;
}

function disposeMedia(node: HTMLElement): void {
  const video = node.querySelector('video');
  if (video) {
    video.pause();
    video.removeAttribute('src');
    video.load();
  }
  for (const url of mediaObjectUrls.get(node) ?? []) URL.revokeObjectURL(url);
  mediaObjectUrls.delete(node);
  node.querySelector('.wr-media-frame')?.remove();
  const off = node.querySelector<HTMLElement>('.wr-media-off');
  if (off) off.hidden = false;
  node.dataset.state = 'off';
}

function refreshMedia(): void {
  mediaObserver?.disconnect();
  mediaObserver = null;
  const body = bodyEl();
  const nodes = els<HTMLElement>('.wr-media');
  if (!nodes.length) return;
  for (const node of nodes) {
    if (!mediaAllowed(node)) {
      if (node.dataset.state !== 'off') disposeMedia(node);
      continue;
    }
    if (node.dataset.state === 'loaded' || node.dataset.state === 'loading') continue;
    if (!('IntersectionObserver' in window)) {
      void hydrateMedia(node);
      continue;
    }
    if (!mediaObserver) {
      mediaObserver = new IntersectionObserver(
        (entries) => {
          for (const entry of entries) {
            if (!entry.isIntersecting) continue;
            mediaObserver?.unobserve(entry.target);
            void hydrateMedia(entry.target as HTMLElement);
          }
        },
        { root: body, rootMargin: '520px 0px' },
      );
    }
    mediaObserver.observe(node);
  }
  updateVideoSettings();
}

function span(className: string, value = ''): HTMLSpanElement {
  const node = document.createElement('span');
  node.className = className;
  if (value) node.textContent = value;
  return node;
}

async function getCachedImage(src: string): Promise<string | null> {
  if (!('caches' in window) || !src) return null;
  try {
    const url = new URL(src, window.location.href).href;
    const cache = await caches.open(MEDIA_CACHE);
    const response = await cache.match(url);
    if (!response) return null;
    const blob = await response.blob();
    return URL.createObjectURL(blob);
  } catch {
    return null;
  }
}

async function cacheImage(src: string): Promise<void> {
  if (!('caches' in window) || !src) return;
  try {
    const url = new URL(src, window.location.href).href;
    const cache = await caches.open(MEDIA_CACHE);
    if (await cache.match(url)) return;
    const response = await fetch(url, { credentials: 'same-origin' });
    if (!response.ok) return;
    const type = response.headers.get('content-type') || '';
    if (!type.startsWith('image/') && !/\.(svg|png|jpe?g|gif|webp|avif)(\?|$)/i.test(url)) return;
    const blob = await response.clone().blob();
    if (blob.size > MEDIA_MAX_IMAGE) return;
    await cache.put(url, response);
    recordMedia(url, blob.size);
  } catch {
    /* cross-origin, private mode or no Cache API: the direct URL still works */
  }
}

async function hydrateMedia(node: HTMLElement): Promise<void> {
  const kind = mediaKind(node);
  if (!kind) return;
  if (kind === 'image' && !state.settings.images) return;
  if (kind === 'video' && !state.settings.videos) return;
  if (node.dataset.state === 'loading' || node.dataset.state === 'loaded') return;
  node.dataset.state = 'loading';
  const src = node.dataset.src || '';
  if (!src) {
    node.dataset.state = 'off';
    return;
  }
  const off = node.querySelector<HTMLElement>('.wr-media-off');
  if (off) off.hidden = true;

  const frame = span('wr-media-frame');
  const head = span('wr-media-head');
  const body = span('wr-media-body');
  const foot = span('wr-media-foot', '╰─');
  const urls: string[] = [];
  mediaObjectUrls.set(node, urls);

  const open = document.createElement('a');
  open.href = src;
  open.target = '_blank';
  open.rel = 'noopener noreferrer';
  open.textContent = 'open ↗';

  if (kind === 'image') {
    const alt = node.dataset.alt || 'image';
    head.textContent = `╭─ image · ${alt}`;
    const img = document.createElement('img');
    img.alt = alt;
    img.loading = 'lazy';
    img.decoding = 'async';
    img.fetchPriority = 'low';
    if (node.dataset.width) img.width = Number(node.dataset.width);
    if (node.dataset.height) img.height = Number(node.dataset.height);
    const cached = await getCachedImage(src);
    if (!mediaAllowed(node) || node.dataset.state !== 'loading') return;
    if (cached) {
      img.src = cached;
      urls.push(cached);
    } else {
      await cacheImage(src);
      if (!mediaAllowed(node) || node.dataset.state !== 'loading') return;
      const stored = await getCachedImage(src);
      if (stored) {
        img.src = stored;
        urls.push(stored);
      } else {
        img.src = src;
      }
    }
    img.addEventListener('load', () => {
      foot.textContent = `╰─ ${img.naturalWidth}×${img.naturalHeight} · `;
      foot.append(open);
    });
    img.addEventListener('error', () => {
      head.textContent = '╭─ image · unavailable';
      img.replaceWith(span('wr-media-error', 'the image could not be loaded'));
      foot.textContent = '╰─ ';
      foot.append(open);
    });
    body.append(img);
    if (!img.width) body.classList.add('wr-media-skeleton');
    img.addEventListener('load', () => body.classList.remove('wr-media-skeleton'));
    img.addEventListener('error', () => body.classList.remove('wr-media-skeleton'));
  } else {
    const label = node.dataset.label || 'video';
    head.textContent = `╭─ video · ${label}`;
    const video = document.createElement('video');
    if (node.dataset.width) video.width = Number(node.dataset.width);
    if (node.dataset.height) video.height = Number(node.dataset.height);
    video.src = src;
    video.controls = state.settings.videoControls;
    video.loop = state.settings.videoLoop;
    video.muted = state.settings.videoMuted || state.settings.videoAutoplay;
    video.autoplay = state.settings.videoAutoplay;
    video.playsInline = true;
    video.preload = 'metadata';
    const poster = node.dataset.poster || '';
    if (poster) {
      video.poster = poster;
      const cachedPoster = await getCachedImage(poster);
      if (cachedPoster) {
        video.poster = cachedPoster;
        urls.push(cachedPoster);
      } else {
        void cacheImage(poster);
      }
    }
    video.addEventListener('loadedmetadata', () => {
      const mins = Math.floor(video.duration / 60);
      const secs = Math.round(video.duration % 60);
      foot.textContent = `╰─ ${mins}:${String(secs).padStart(2, '0')} · stream · `;
      foot.append(open);
    });
    video.addEventListener('error', () => {
      head.textContent = '╭─ video · unavailable';
      video.replaceWith(span('wr-media-error', 'the video could not be loaded'));
      foot.textContent = '╰─ ';
      foot.append(open);
    });
    body.append(video);
  }

  if (!mediaAllowed(node) || node.dataset.state !== 'loading') return;
  frame.append(head, body, foot);
  node.append(frame);
  node.dataset.state = 'loaded';
}

function updateVideoSettings(): void {
  const videos = els<HTMLVideoElement>('.wr-media video');
  for (const video of videos) {
    video.controls = state.settings.videoControls;
    video.loop = state.settings.videoLoop;
    video.muted = state.settings.videoMuted || state.settings.videoAutoplay;
    if (state.settings.videoAutoplay) void video.play().catch(() => undefined);
  }
}

async function clearMediaCache(): Promise<void> {
  try {
    if ('caches' in window) {
      const cache = await caches.open(MEDIA_CACHE);
      const keys = await cache.keys();
      await Promise.all(keys.map((key) => cache.delete(key)));
    }
  } catch {
    /* no Cache API */
  }
  try {
    localStorage.removeItem(MEDIA_MANIFEST);
  } catch {
    /* private mode */
  }
  updateMediaCacheRow();
  toast('✓ media cache cleared');
}

// ---------------------------------------------------------------- footer

function defaultHints(): string {
  return `<span><b>o</b> open</span> <span><b>/</b> search</span> <span><b>H</b> back</span> <span><b>L</b> fwd</span> <span class="wide"><b>f</b> links</span> <span class="medium"><b>t</b> index</span> <span class="medium"><b>r</b> reload</span> <span><b>m</b> menu</span> <span class="wide"><b>q</b> start</span>`;
}
function panelHints(): string {
  return `<span><b>↑↓</b> choose</span> <span><b>enter</b> accept</span> <span><b>/</b> filter</span> <span><b>esc</b> cancel</span>`;
}
function findHints(): string {
  return `<span><b>type</b> to search</span> <span><b>↑↓</b> next</span> <span><b>enter</b> accept</span> <span><b>esc</b> cancel</span>`;
}
function hintsHints(): string {
  return `<span><b>type a label</b> to follow a link</span> <span><b>esc</b> cancel</span>`;
}

function updateFooter(): void {
  const hints = el<HTMLElement>('[data-wr-hints]');
  const find = el<HTMLElement>('[data-wr-find]');
  if (!hints || !find) return;
  if (state.mode === 'find') {
    hints.hidden = true;
    find.hidden = false;
  } else {
    find.hidden = true;
    hints.hidden = false;
  }
  if (state.mode === 'read') hints.innerHTML = defaultHints();
  else if (state.mode === 'find') hints.innerHTML = findHints();
  else if (state.mode === 'hints') hints.innerHTML = hintsHints();
  else if (state.mode === 'help') hints.innerHTML = `<span><b>esc</b> close</span> <span><b>?</b> close</span>`;
  else hints.innerHTML = panelHints();
}

function updateChip(): void {
  const chip = el<HTMLElement>('[data-wr-chip]');
  if (!chip) return;
  if (state.findMarks.length && state.findQuery) {
    chip.hidden = false;
    chip.textContent = `«${state.findQuery}» ${state.findIndex + 1}/${state.findMarks.length}`;
  } else {
    chip.hidden = true;
    chip.textContent = '';
  }
}

function updateChrome(): void {
  const body = bodyEl();
  if (!body) return;
  const maxTop = body.scrollHeight - body.clientHeight;
  const pct = readPercent(body.scrollTop, maxTop);
  const pctEl = el<HTMLElement>('[data-wr-pct]');
  const barEl = el<HTMLElement>('[data-wr-bar]');
  if (pctEl) pctEl.textContent = maxTop <= 0 ? 'all visible' : `${pct}%`;
  if (barEl) {
    if (maxTop <= 0) {
      barEl.innerHTML = '';
    } else {
      const cells = progressCells(window.innerWidth);
      const bar = progressBar(cells, pct, state.settings.braille);
      barEl.innerHTML = `<span class="full">${bar.full}</span><span class="empty">${bar.empty}</span>`;
    }
  }
  updateScrollbar();
  updateChip();
}

function updateScrollbar(): void {
  const body = bodyEl();
  const scroll = el<HTMLElement>('[data-wr-scroll]');
  if (!body || !scroll) return;
  const style = getComputedStyle(scroll);
  const padTop = parseFloat(style.paddingTop) || 0;
  const available = Math.max(0, scroll.clientHeight - padTop);
  const rows = Math.max(1, Math.floor(available / lineHeight(body)));
  const chars = scrollbarChars(rows, body.scrollHeight, body.clientHeight, body.scrollTop, state.settings.braille);
  if (!chars.length) {
    scroll.innerHTML = '';
    return;
  }
  scroll.innerHTML = chars
    .map((c) => `<span class="${c === TRACK || c === PLAIN_TRACK ? 'track' : 'thumb'}">${c}</span>`)
    .join('');
}

let chromeFrame = 0;
function scheduleChrome(): void {
  if (chromeFrame) return;
  chromeFrame = requestAnimationFrame(() => {
    chromeFrame = 0;
    rememberScroll();
    updateChrome();
  });
}

function rememberScroll(): void {
  const body = bodyEl();
  if (!body) return;
  state.lastPath = window.location.pathname;
  state.lastScroll = body.scrollTop;
}

// ---------------------------------------------------------------- toast

function toast(message: string, warn = false): void {
  let node = el<HTMLElement>('[data-wr-toast]');
  if (!node) {
    node = document.createElement('div');
    node.className = 'wr-toast';
    node.dataset.wrToast = '';
    node.setAttribute('role', 'status');
    node.setAttribute('aria-live', 'polite');
    document.body.appendChild(node);
  }
  node.textContent = message;
  node.classList.toggle('warn', warn);
  node.hidden = false;
  window.clearTimeout(state.toastTimer);
  state.toastTimer = window.setTimeout(() => {
    node!.hidden = true;
  }, 2400);
}

// ---------------------------------------------------------------- clipboard

async function copyText(text: string): Promise<void> {
  if (!text) return;
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    const area = document.createElement('textarea');
    area.value = text;
    area.setAttribute('readonly', '');
    area.style.position = 'fixed';
    area.style.left = '-9999px';
    document.body.appendChild(area);
    area.select();
    try {
      document.execCommand('copy');
    } finally {
      area.remove();
    }
  }
}

function copySelectionOrURL(): void {
  const selected = window.getSelection()?.toString() ?? '';
  const text = selected || window.location.href;
  void copyText(text).then(() => toast(selected ? '✓ copied the selection' : '✓ copied the address'));
}

function copyCodeBlock(): void {
  const body = bodyEl();
  if (!body) return;
  const pres = els<HTMLPreElement>('.wr-doc pre', body);
  if (!pres.length) {
    toast('no code block on this screen', true);
    return;
  }
  const view = body.getBoundingClientRect();
  const visible =
    pres.find((pre) => {
      const rect = pre.getBoundingClientRect();
      return rect.bottom >= view.top && rect.top <= view.bottom;
    }) ?? pres[0]!;
  const lines = els<HTMLElement>('.line', visible).map((line) => line.textContent ?? '');
  const text = (lines.length ? lines.join('\n') : visible.textContent ?? '').replace(/\s+$/, '');
  if (!text.trim()) {
    toast('this code block is empty', true);
    return;
  }
  void copyText(text).then(() => toast(`✓ copied code block · ${text.split('\n').length} lines`));
}

function copyPageMarkdown(): void {
  const node = el<HTMLScriptElement>('script[data-wr-markdown]');
  let text = '';
  if (node?.textContent) {
    try {
      text = JSON.parse(node.textContent) as string;
    } catch {
      text = '';
    }
  }
  if (!text) {
    const doc = el<HTMLElement>('.wr-doc');
    text = doc?.innerText ?? '';
  }
  if (!text.trim()) {
    toast('there is no page to copy', true);
    return;
  }
  void copyText(text).then(() => toast(`✓ copied page as Markdown · ${text.length} chars`));
}

// ---------------------------------------------------------------- panels

function layer(): HTMLElement | null {
  return el<HTMLElement>('[data-wr-layer]');
}
function openPanel(name: Exclude<Mode, 'read' | 'find' | 'hints'>): void {
  clearHints();
  state.mode = name;
  state.filterOpen = false;
  state.filter = '';
  const wrap = layer();
  if (!wrap) return;
  wrap.hidden = false;
  els<HTMLElement>('.wr-panel', wrap).forEach((panel) => {
    panel.hidden = panel.dataset.panel !== name;
  });
  const panel = el<HTMLElement>(`[data-panel="${name}"]`, wrap);
  if (!panel) return;
  hidePanelFilter();
  if (name === 'toc') {
    buildToc();
    state.tocIndex = 0;
    selectRows('toc');
  } else if (name === 'settings') {
    buildSettings();
    state.settingsIndex = 0;
    selectRows('settings');
  } else if (name === 'menu') {
    state.menuIndex = 0;
    selectRows('menu');
  } else if (name === 'omni') {
    void openOmni();
  } else if (name === 'help') {
    panel.querySelector<HTMLElement>('.wr-panel-body')?.scrollTo({ top: 0 });
  }
  updateFooter();
  if (name !== 'omni') {
    panel.tabIndex = -1;
    panel.focus({ preventScroll: true });
  }
}

function closePanel(): void {
  clearHints();
  state.mode = 'read';
  state.filterOpen = false;
  state.filter = '';
  const wrap = layer();
  if (wrap) wrap.hidden = true;
  els<HTMLElement>('.wr-panel').forEach((panel) => (panel.hidden = true));
  updateFooter();
  bodyEl()?.focus({ preventScroll: true });
}

function panelRows(name: Mode): HTMLElement[] {
  const panel = el<HTMLElement>(`[data-panel="${name}"]`);
  if (!panel) return [];
  return els<HTMLElement>('.wr-row, .wr-settings-row', panel).filter((row) => !row.hidden);
}

function selectedIndex(name: Mode): number {
  if (name === 'menu') return state.menuIndex;
  if (name === 'toc') return state.tocIndex;
  return state.settingsIndex;
}
function setSelectedIndex(name: Mode, index: number): void {
  if (name === 'menu') state.menuIndex = index;
  else if (name === 'toc') state.tocIndex = index;
  else state.settingsIndex = index;
}

function selectRows(name: Mode): void {
  const rows = panelRows(name);
  if (!rows.length) return;
  let index = clamp(selectedIndex(name), 0, rows.length - 1);
  setSelectedIndex(name, index);
  rows.forEach((row, i) => {
    row.classList.toggle('sel', i === index);
    row.setAttribute('aria-selected', i === index ? 'true' : 'false');
  });
  const row = rows[index]!;
  row.scrollIntoView({ block: 'nearest' });
  const help = el<HTMLElement>(`[data-panel="${name}"] .wr-panel-help`);
  if (help) help.textContent = row.dataset.desc || '';
}

function moveSelection(name: Mode, dir: 1 | -1): void {
  const rows = panelRows(name);
  if (!rows.length) return;
  const next = (selectedIndex(name) + dir + rows.length) % rows.length;
  setSelectedIndex(name, next);
  selectRows(name);
}

function chooseSelection(name: Mode): void {
  const rows = panelRows(name);
  const row = rows[selectedIndex(name)];
  if (!row) return;
  activateRow(row);
}

function activateRow(row: HTMLElement): void {
  const action = row.dataset.action;
  if (action) {
    runAction(action, row);
    return;
  }
  const href = row.dataset.href || (row instanceof HTMLAnchorElement ? row.href : '');
  if (href) navigate(href);
}

function runAction(action: string, row?: HTMLElement): void {
  switch (action) {
    case 'top':
      scrollTop();
      closePanel();
      break;
    case 'bottom':
      scrollBottom();
      closePanel();
      break;
    case 'toc':
      openPanel('toc');
      break;
    case 'find':
      closePanel();
      openFind();
      break;
    case 'omni':
      openPanel('omni');
      break;
    case 'help':
      openPanel('help');
      break;
    case 'settings':
      openPanel('settings');
      break;
    case 'hints':
      closePanel();
      startHints();
      break;
    case 'back':
      closePanel();
      history.back();
      break;
    case 'forward':
      closePanel();
      history.forward();
      break;
    case 'home':
      closePanel();
      navigate(withBase('/'));
      break;
    case 'copy-url':
      closePanel();
      void copyText(window.location.href).then(() => toast('✓ copied the address'));
      break;
    case 'copy-install':
      closePanel();
      void copyText(INSTALL).then(() => toast('✓ copied the install command'));
      break;
    case 'copy-page':
      closePanel();
      copyPageMarkdown();
      break;
    case 'copy-code':
      closePanel();
      copyCodeBlock();
      break;
    case 'clear-media-cache':
      closePanel();
      void clearMediaCache();
      break;
    case 'reload':
      closePanel();
      window.location.reload();
      break;
    case 'jump':
      if (row?.dataset.target) {
        jumpTo(row.dataset.target);
        closePanel();
      }
      break;
    default:
      break;
  }
}

function navigate(href: string): void {
  if (!href) return;
  if (/^https?:\/\//i.test(href) && new URL(href).origin !== window.location.origin) {
    window.open(href, '_blank', 'noopener');
    return;
  }
  void astroNavigate(href);
}

function jumpTo(id: string): void {
  const target = document.getElementById(id);
  if (!target) return;
  target.scrollIntoView({ block: 'start', behavior: 'auto' });
  try {
    history.replaceState(null, '', `#${id}`);
  } catch {
    /* ignore */
  }
}

// ---------------------------------------------------------------- menu / toc / settings

function buildToc(): void {
  const doc = el<HTMLElement>('.wr-doc');
  const headings = doc ? els<HTMLElement>('h1, h2, h3, h4', doc) : [];
  const container = el<HTMLElement>('[data-toc-body]');
  if (!container) return;
  if (!headings.length) {
    container.innerHTML = '<div class="wr-empty">no sections</div>';
    return;
  }
  container.innerHTML = headings
    .map((heading, i) => {
      const depth = Math.min(Number(heading.tagName.slice(1)) - 1, 4);
      if (!heading.id) heading.id = `section-${i}`;
      const text = heading.textContent?.trim() || heading.id;
      return `<button type="button" class="wr-row" data-action="jump" data-target="${escapeHtml(heading.id)}" data-depth="${depth}" data-desc="Jump to this section"><span class="wr-row-label">${escapeHtml(text)}</span><span class="wr-row-key">${depth === 0 ? 'top' : '#'}</span></button>`;
    })
    .join('');
}

function buildSettings(): void {
  const container = el<HTMLElement>('[data-settings-body]');
  if (!container) return;
  const rows: string[] = [];
  const s = state.settings;
  rows.push(
    settingRow('Accent', s.accent, 'The accent paints headings, links, the scrollbar and the progress.', 'accent'),
    settingRow('Reading width', s.width, 'How wide the text column is, like the Width setting in wr.', 'width'),
    settingRow('Spacing', s.spacing, 'Vertical rhythm between paragraphs and blocks.', 'spacing'),
    settingRow('Heading rules', s.headingRules ? 'on' : 'off', 'The braille rule under the first two heading levels.', 'headingRules'),
    settingRow('Code frames', s.codeFrames ? 'on' : 'off', 'The labelled frame around code blocks.', 'codeFrames'),
    settingRow('Braille', s.braille ? 'on' : 'off', 'Braille headings, scrollbar and progress bar.', 'braille'),
    settingRow('Center text', s.center ? 'on' : 'off', 'Center the reading column in a wide window.', 'center'),
    settingRow('Images', s.images ? 'on' : 'off', 'Show page images in a lazy, cached frame. Off by default.', 'images'),
    settingRow('Videos', s.videos ? 'on' : 'off', 'Show page videos with native controls. Off by default.', 'videos'),
    settingRow('Image fit', s.imageFit, 'Contain keeps the whole image; cover fills the frame and crops.', 'imageFit'),
    settingRow('Image width', s.imageWidth, 'How much of the reading column an image may take.', 'imageWidth'),
    settingRow('Video autoplay', s.videoAutoplay ? 'on' : 'off', 'Play videos when they enter the viewport; always muted.', 'videoAutoplay'),
    settingRow('Video loop', s.videoLoop ? 'on' : 'off', 'Restart a video when it ends.', 'videoLoop'),
    settingRow('Video muted', s.videoMuted ? 'on' : 'off', 'Start videos without sound.', 'videoMuted'),
    settingRow('Video controls', s.videoControls ? 'on' : 'off', 'Show the browser’s play, seek and volume controls.', 'videoControls'),
    mediaCacheRow(),
    `<button type="button" class="wr-settings-row" data-setting="reset" data-desc="Put every option back to its default."><span class="s-label">Reset to defaults</span><span class="s-value">enter</span></button>`,
  );
  container.innerHTML = rows.join('');
  updateMediaCacheRow();
}

function settingRow(label: string, value: string, desc: string, key: string): string {
  return `<button type="button" class="wr-settings-row" data-setting="${key}" data-desc="${escapeHtml(desc)}"><span class="s-label">${escapeHtml(label)}</span><span class="s-value">${escapeHtml(value)}</span></button>`;
}

function mediaCacheRow(): string {
  const { count, bytes } = mediaCacheSummary();
  const value = count ? `${count} images · ${humanBytes(bytes)}` : 'empty';
  return `<button type="button" class="wr-settings-row" data-setting="mediaCache" data-desc="Images and video posters stored by this browser. Enter clears them."><span class="s-label">Media cache</span><span class="s-value" data-media-cache-value>${value}</span></button>`;
}

function cycleSetting(key: string): void {
  const s = state.settings;
  if (key === 'reset') {
    state.settings = { ...DEFAULTS };
    applySettings();
    buildSettings();
    selectRows('settings');
    toast('settings reset');
    return;
  }
  switch (key) {
    case 'accent': {
      const i = ACCENTS.indexOf(s.accent as (typeof ACCENTS)[number]);
      s.accent = ACCENTS[(i + 1) % ACCENTS.length]!;
      break;
    }
    case 'width': {
      const values: Settings['width'][] = ['narrow', 'normal', 'wide', 'full'];
      s.width = values[(values.indexOf(s.width) + 1) % values.length]!;
      break;
    }
    case 'spacing': {
      const values: Settings['spacing'][] = ['0', '1', '2'];
      s.spacing = values[(values.indexOf(s.spacing) + 1) % values.length]!;
      break;
    }
    case 'headingRules':
      s.headingRules = !s.headingRules;
      break;
    case 'codeFrames':
      s.codeFrames = !s.codeFrames;
      break;
    case 'braille':
      s.braille = !s.braille;
      break;
    case 'center':
      s.center = !s.center;
      break;
    case 'images':
      s.images = !s.images;
      break;
    case 'videos':
      s.videos = !s.videos;
      break;
    case 'imageFit':
      s.imageFit = s.imageFit === 'contain' ? 'cover' : 'contain';
      break;
    case 'imageWidth': {
      const values: Settings['imageWidth'][] = ['full', 'wide', 'half'];
      s.imageWidth = values[(values.indexOf(s.imageWidth) + 1) % values.length]!;
      break;
    }
    case 'videoAutoplay':
      s.videoAutoplay = !s.videoAutoplay;
      break;
    case 'videoLoop':
      s.videoLoop = !s.videoLoop;
      break;
    case 'videoMuted':
      s.videoMuted = !s.videoMuted;
      break;
    case 'videoControls':
      s.videoControls = !s.videoControls;
      break;
    case 'mediaCache':
      void clearMediaCache();
      return;
    default:
      return;
  }
  applySettings();
  const index = state.settingsIndex;
  buildSettings();
  state.settingsIndex = index;
  selectRows('settings');
  if (key === 'braille') updateChrome();
}

// ---------------------------------------------------------------- find

function openFind(): void {
  if (state.mode !== 'read') closePanel();
  state.mode = 'find';
  const form = el<HTMLElement>('[data-wr-find]');
  const input = el<HTMLInputElement>('[data-wr-find-input]');
  if (!form || !input) return;
  form.hidden = false;
  input.value = state.findQuery;
  updateFooter();
  input.focus();
  input.select();
}

function closeFind(clear: boolean): void {
  if (clear) {
    state.findQuery = '';
    const input = el<HTMLInputElement>('[data-wr-find-input]');
    if (input) input.value = '';
    clearFind();
  }
  state.mode = 'read';
  updateFooter();
  updateChip();
  bodyEl()?.focus({ preventScroll: true });
}

function clearFind(): void {
  els<HTMLElement>('mark.wr-hit').forEach((mark) => {
    const parent = mark.parentNode;
    if (!parent) return;
    parent.replaceChild(document.createTextNode(mark.textContent ?? ''), mark);
    parent.normalize();
  });
  state.findMarks = [];
  state.findIndex = -1;
}

function runFind(): void {
  clearFind();
  const query = state.findQuery;
  if (!query.trim()) {
    updateChip();
    const count = el<HTMLElement>('[data-wr-find-count]');
    if (count) count.textContent = '';
    return;
  }
  const root = el<HTMLElement>('.wr-doc') || bodyEl();
  if (!root) return;
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
    acceptNode(node: Node): number {
      const parent = node.parentElement;
      if (!parent) return NodeFilter.FILTER_REJECT;
      if (parent.closest('script, style, noscript, .wr-panel, .wr-toast, .wr-pager')) {
        return NodeFilter.FILTER_REJECT;
      }
      return node.nodeValue?.trim() ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_REJECT;
    },
  });
  const nodes: Text[] = [];
  while (walker.nextNode()) nodes.push(walker.currentNode as Text);
  const needle = query.toLowerCase();
  for (const node of nodes) {
    const text = node.nodeValue ?? '';
    const lower = text.toLowerCase();
    let from = 0;
    let at = lower.indexOf(needle, from);
    if (at === -1) continue;
    const fragment = document.createDocumentFragment();
    while (at !== -1) {
      if (at > from) fragment.appendChild(document.createTextNode(text.slice(from, at)));
      const mark = document.createElement('mark');
      mark.className = 'wr-hit';
      mark.textContent = text.slice(at, at + query.length);
      fragment.appendChild(mark);
      from = at + query.length;
      at = lower.indexOf(needle, from);
    }
    if (from < text.length) fragment.appendChild(document.createTextNode(text.slice(from)));
    node.parentNode?.replaceChild(fragment, node);
  }
  state.findMarks = els<HTMLElement>('mark.wr-hit', root);
  state.findIndex = state.findMarks.length ? 0 : -1;
  if (state.findIndex >= 0) focusMatch(0, false);
  updateFindCount();
  updateChrome();
}

function focusMatch(index: number, scroll = true): void {
  if (!state.findMarks.length) return;
  state.findIndex = (index + state.findMarks.length) % state.findMarks.length;
  state.findMarks.forEach((mark, i) => mark.classList.toggle('current', i === state.findIndex));
  if (scroll) state.findMarks[state.findIndex]?.scrollIntoView({ block: 'center', behavior: 'auto' });
  updateFindCount();
}

function updateFindCount(): void {
  const count = el<HTMLElement>('[data-wr-find-count]');
  if (count) {
    if (!state.findQuery) count.textContent = '';
    else if (!state.findMarks.length) count.textContent = 'no results';
    else count.textContent = `${state.findIndex + 1}/${state.findMarks.length}`;
  }
  updateChip();
}

// ---------------------------------------------------------------- hints

function clearHints(): void {
  els<HTMLElement>('[data-hint]').forEach((a) => delete a.dataset.hint);
  state.hints = [];
  state.hintInput = '';
}

function startHints(): void {
  closePanel();
  clearHints();
  const body = bodyEl();
  if (!body) return;
  const links = els<HTMLAnchorElement>('.wr-doc a[href], .wr-start a[href]', body).filter(
    (a) => a.offsetParent !== null && !a.closest('.wr-pager, .wr-source'),
  );
  if (!links.length) {
    toast('no links on this screen', true);
    return;
  }
  links.forEach((link, i) => {
    const label = hintLabel(i);
    link.dataset.hint = label;
    state.hints.push({ label, href: link.href });
  });
  state.mode = 'hints';
  state.hintInput = '';
  updateFooter();
  updateChrome();
}

function hintLabel(i: number): string {
  if (i < HINT_ALPHABET.length) return HINT_ALPHABET[i]!;
  const group = Math.floor(i / HINT_ALPHABET.length) - 1;
  return HINT_ALPHABET[group % HINT_ALPHABET.length]! + HINT_ALPHABET[i % HINT_ALPHABET.length]!;
}

function followHint(key: string): void {
  const next = state.hintInput + key;
  const exact = state.hints.find((hint) => hint.label === next);
  if (exact) {
    const href = exact.href;
    clearHints();
    state.mode = 'read';
    updateFooter();
    navigate(href);
    return;
  }
  if (state.hints.some((hint) => hint.label.startsWith(next))) {
    state.hintInput = next;
    return;
  }
  state.hintInput = '';
}

// ---------------------------------------------------------------- omni

function loadIndex(): Promise<SearchEntry[]> {
  if (!indexPromise) {
    indexPromise = fetch(withBase('/docs/search-index.json'))
      .then((response) => (response.ok ? response.json() : []))
      .then((data: SearchEntry[]) => {
        return data;
      })
      .catch(() => []);
  }
  return indexPromise;
}

async function openOmni(): Promise<void> {
  const input = el<HTMLInputElement>('[data-omni-input]');
  const results = el<HTMLElement>('[data-omni-results]');
  if (!input || !results) return;
  input.value = '';
  state.omniIndex = 0;
  results.innerHTML = '<div class="wr-empty">loading the index…</div>';
  input.focus();
  const index = await loadIndex();
  const initial = index.slice(0, 8);
  renderOmni(initial.map((entry) => ({ title: entry.title, where: `${entry.doc} / ${entry.section}`, href: entry.url, snippet: '' })));
  input.focus();
}

interface OmniResult {
  title: string;
  where: string;
  href: string;
  snippet: string;
}

function renderOmni(results: OmniResult[]): void {
  const box = el<HTMLElement>('[data-omni-results]');
  if (!box) return;
  if (!results.length) {
    box.innerHTML = '<div class="wr-empty">no results. enter opens the search page.</div>';
    state.omniIndex = 0;
    return;
  }
  box.innerHTML = results
    .map(
      (result, i) =>
        `<a class="wr-result${i === state.omniIndex ? ' sel' : ''}" href="${escapeHtml(result.href)}" data-omni-index="${i}"><div class="r-title">${escapeHtml(result.title)}</div><div class="r-where">${escapeHtml(result.where)}</div>${result.snippet ? `<div class="r-snippet">${result.snippet}</div>` : ''}</a>`,
    )
    .join('');
}

function updateOmni(): void {
  const input = el<HTMLInputElement>('[data-omni-input]');
  if (!input) return;
  const query = input.value.trim();
  state.omniIndex = 0;
  if (!query) {
    void loadIndex().then((index) => renderOmni(index.slice(0, 8).map((entry) => ({ title: entry.title, where: `${entry.doc} / ${entry.section}`, href: entry.url, snippet: '' }))));
    return;
  }
  const commands: OmniResult[] = [
    { title: 'Start page', where: 'wr', href: withBase('/'), snippet: '' },
    { title: 'Documentation', where: 'docs', href: withBase('/docs/'), snippet: '' },
    { title: 'Search the docs', where: '/', href: withBase(`/docs/search/?q=${encodeURIComponent(query)}`), snippet: '' },
    { title: 'GitHub', where: 'external', href: 'https://github.com/sazardev/wr', snippet: '' },
    { title: 'Releases', where: 'external', href: 'https://github.com/sazardev/wr/releases', snippet: '' },
  ].filter((item) => item.title.toLowerCase().includes(query.toLowerCase()));
  void loadIndex().then((index) => {
    const hits = searchEntries(index, query, { limit: 8 }).map((hit) => ({
      title: hit.entry.title,
      where: `${hit.entry.doc} / ${hit.entry.section}`,
      href: hit.entry.url,
      snippet: highlightTerms(hit.snippet, query),
    }));
    renderOmni([...commands, ...hits].slice(0, 10));
  });
}

function moveOmni(dir: 1 | -1): void {
  const results = els<HTMLElement>('[data-omni-results] .wr-result');
  if (!results.length) return;
  state.omniIndex = (state.omniIndex + dir + results.length) % results.length;
  results.forEach((row, i) => row.classList.toggle('sel', i === state.omniIndex));
  results[state.omniIndex]?.scrollIntoView({ block: 'nearest' });
}

function chooseOmni(): void {
  const input = el<HTMLInputElement>('[data-omni-input]');
  const results = els<HTMLAnchorElement>('[data-omni-results] .wr-result');
  const chosen = results[state.omniIndex];
  if (chosen) {
    navigate(chosen.href);
    return;
  }
  const query = input?.value.trim() ?? '';
  if (!query) return;
  if (/^https?:\/\//i.test(query)) {
    window.location.assign(query);
    return;
  }
  navigate(withBase(`/docs/search/?q=${encodeURIComponent(query)}`));
}

function highlightTerms(text: string, query: string): string {
  const terms = tokenize(query).map(escapeRegExp);
  if (!terms.length) return escapeHtml(text);
  return escapeHtml(text).replace(new RegExp(`(${terms.join('|')})`, 'gi'), '<mark class="wr-hit">$1</mark>');
}

// ---------------------------------------------------------------- search page

function startSearchPage(): void {
  const page = el<HTMLElement>('[data-search-page]');
  if (!page) return;
  const form = el<HTMLFormElement>('[data-search-form]', page);
  const input = el<HTMLInputElement>('[data-search-input]', page);
  const results = el<HTMLElement>('[data-search-results]', page);
  const status = el<HTMLElement>('[data-search-status]', page);
  if (!form || !input || !results) return;
  let selected = 0;

  const params = new URLSearchParams(window.location.search);
  input.value = params.get('q') ?? '';

  const render = async (): Promise<void> => {
    const query = input.value.trim();
    try {
      history.replaceState(null, '', query ? `?q=${encodeURIComponent(query)}` : window.location.pathname);
    } catch {
      /* ignore */
    }
    if (!query) {
      results.innerHTML = '<div class="wr-empty">Type a word: links, keys, cache, install…</div>';
      if (status) status.textContent = '';
      updateChrome();
      return;
    }
    const index = await loadIndex();
    const hits = searchEntries(index, query, { limit: 30 });
    if (status) status.textContent = hits.length ? `${hits.length} result${hits.length === 1 ? '' : 's'}` : 'no results';
    if (!hits.length) {
      results.innerHTML = '<div class="wr-empty">No page matches that. Try a shorter word.</div>';
      updateChrome();
      return;
    }
    selected = 0;
    results.innerHTML = hits
      .map(
        (hit, i) =>
          `<a class="wr-result${i === 0 ? ' sel' : ''}" href="${escapeHtml(hit.entry.url)}" data-result="${i}"><div class="r-title">${escapeHtml(hit.entry.title)}</div><div class="r-where">${escapeHtml(hit.entry.doc)} / ${escapeHtml(hit.entry.section)}</div><div class="r-snippet">${highlightTerms(hit.snippet, query)}</div></a>`,
      )
      .join('');
    updateChrome();
  };

  input.addEventListener('input', () => void render());
  form.addEventListener('submit', (event) => {
    event.preventDefault();
    const rows = els<HTMLAnchorElement>('[data-result]', results);
    if (rows[selected]) navigate(rows[selected]!.href);
  });
  input.addEventListener('keydown', (event) => {
    const rows = els<HTMLAnchorElement>('[data-result]', results);
    if (!rows.length) return;
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      selected = (selected + (event.key === 'ArrowDown' ? 1 : -1) + rows.length) % rows.length;
      rows.forEach((row, i) => row.classList.toggle('sel', i === selected));
      rows[selected]?.scrollIntoView({ block: 'nearest' });
    } else if (event.key === 'Enter') {
      event.preventDefault();
      navigate(rows[selected]!.href);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      input.value = '';
      void render();
      input.focus();
    }
  });
  void render();
}

// ---------------------------------------------------------------- keys

function isTyping(target: EventTarget | null): boolean {
  const node = target as HTMLElement | null;
  return Boolean(node?.closest('input, textarea, select, video, audio, [contenteditable="true"]'));
}

function onKeyDown(event: KeyboardEvent): void {
  if (event.defaultPrevented || isTyping(event.target)) return;
  const key = event.key;
  const lower = key.toLowerCase();

  if (state.mode === 'hints') {
    if (key === 'Escape') {
      event.preventDefault();
      clearHints();
      state.mode = 'read';
      updateFooter();
      return;
    }
    if (key.length === 1 && /^[a-z0-9]$/i.test(key)) {
      event.preventDefault();
      followHint(key.toLowerCase());
      return;
    }
    return;
  }

  if (state.mode === 'help') {
    if (key === 'Escape' || key === '?') {
      event.preventDefault();
      closePanel();
      return;
    }
    if (key === 'q') {
      event.preventDefault();
      closePanel();
      navigate(withBase('/'));
      return;
    }
    const panelBody = el<HTMLElement>('[data-panel="help"] .wr-panel-body');
    if (panelBody) {
      const page = panelBody.clientHeight * 0.9;
      if (key === 'ArrowDown' || lower === 'j') { event.preventDefault(); panelBody.scrollBy({ top: 32 }); }
      else if (key === 'ArrowUp' || lower === 'k') { event.preventDefault(); panelBody.scrollBy({ top: -32 }); }
      else if (key === 'PageDown' || key === ' ') { event.preventDefault(); panelBody.scrollBy({ top: page }); }
      else if (key === 'PageUp') { event.preventDefault(); panelBody.scrollBy({ top: -page }); }
      else if (key === 'Home' || lower === 'g') { event.preventDefault(); panelBody.scrollTop = 0; }
      else if (key === 'End' || key === 'G') { event.preventDefault(); panelBody.scrollTop = panelBody.scrollHeight; }
    }
    return;
  }

  if (state.mode === 'menu' || state.mode === 'toc' || state.mode === 'settings') {
    if (key === 'q') {
      event.preventDefault();
      closePanel();
      navigate(withBase('/'));
      return;
    }
    if (key === 'Escape') {
      event.preventDefault();
      if (state.filterOpen) {
        state.filterOpen = false;
        state.filter = '';
        hidePanelFilter();
        filterRows(state.mode);
      } else {
        closePanel();
      }
      return;
    }
    if (key === 'ArrowDown' || lower === 'j') {
      event.preventDefault();
      moveSelection(state.mode, 1);
      return;
    }
    if (key === 'ArrowUp' || lower === 'k') {
      event.preventDefault();
      moveSelection(state.mode, -1);
      return;
    }
    if (key === 'Home') {
      event.preventDefault();
      setSelectedIndex(state.mode, 0);
      selectRows(state.mode);
      return;
    }
    if (key === 'End') {
      event.preventDefault();
      const rows = panelRows(state.mode);
      setSelectedIndex(state.mode, rows.length - 1);
      selectRows(state.mode);
      return;
    }
    if (key === 'Enter') {
      event.preventDefault();
      if (state.mode === 'settings') {
        const row = panelRows('settings')[state.settingsIndex];
        if (row?.dataset.setting) cycleSetting(row.dataset.setting);
      } else {
        chooseSelection(state.mode);
      }
      return;
    }
    if (state.mode === 'settings' && (key === 'ArrowRight' || key === 'ArrowLeft')) {
      event.preventDefault();
      const row = panelRows('settings')[state.settingsIndex];
      if (row?.dataset.setting) cycleSetting(row.dataset.setting);
      return;
    }
    if (key === '/') {
      event.preventDefault();
      openPanelFilter();
      return;
    }
    if (/^[1-9]$/.test(key) && state.mode === 'menu') {
      event.preventDefault();
      setSelectedIndex('menu', Number(key) - 1);
      selectRows('menu');
      return;
    }
    if (key === '0' && state.mode === 'menu') {
      event.preventDefault();
      setSelectedIndex('menu', 9);
      selectRows('menu');
      return;
    }
    return;
  }

  if (state.mode === 'omni') {
    if (key === 'Escape') {
      event.preventDefault();
      closePanel();
    }
    return;
  }

  if (state.mode === 'find') {
    if (key === 'Escape') {
      event.preventDefault();
      closeFind(true);
    }
    return;
  }

  // read mode
  if ((event.ctrlKey || event.metaKey) && lower === 'y') {
    event.preventDefault();
    copyPageMarkdown();
    return;
  }
  switch (key) {
    case 'j':
    case 'ArrowDown':
      event.preventDefault();
      scrollByLines(1);
      break;
    case 'k':
    case 'ArrowUp':
      event.preventDefault();
      scrollByLines(-1);
      break;
    case 'd':
      event.preventDefault();
      scrollByPage(1);
      break;
    case 'u':
      event.preventDefault();
      scrollByPage(-1);
      break;
    case ' ':
    case 'PageDown':
      event.preventDefault();
      scrollByPage(1);
      break;
    case 'b':
    case 'PageUp':
      event.preventDefault();
      scrollByPage(-1);
      break;
    case 'g':
    case 'Home':
      event.preventDefault();
      scrollTop();
      break;
    case 'G':
    case 'End':
      event.preventDefault();
      scrollBottom();
      break;
    case '/':
      event.preventDefault();
      openFind();
      break;
    case 'n':
      event.preventDefault();
      if (state.findMarks.length) focusMatch(state.findIndex + 1);
      else toast('no active search; press / first', true);
      break;
    case 'N':
      event.preventDefault();
      if (state.findMarks.length) focusMatch(state.findIndex - 1);
      else toast('no active search; press / first', true);
      break;
    case 'f':
      event.preventDefault();
      startHints();
      break;
    case 't':
      event.preventDefault();
      openPanel('toc');
      break;
    case 'm':
    case '?':
      event.preventDefault();
      openPanel(key === 'm' ? 'menu' : 'help');
      break;
    case ',':
      event.preventDefault();
      openPanel('settings');
      break;
    case 'o':
      event.preventDefault();
      openPanel('omni');
      break;
    case 'H':
      event.preventDefault();
      history.back();
      break;
    case 'L':
      event.preventDefault();
      history.forward();
      break;
    case 'r':
      event.preventDefault();
      window.location.reload();
      break;
    case 'q':
      event.preventDefault();
      navigate(withBase('/'));
      break;
    case 'y':
      event.preventDefault();
      copySelectionOrURL();
      break;
    case 'Y':
      event.preventDefault();
      copyCodeBlock();
      break;
    case 'U':
      event.preventDefault();
      window.open('https://github.com/sazardev/wr/releases', '_blank', 'noopener');
      break;
    case '[': {
      event.preventDefault();
      const prev = el<HTMLAnchorElement>('[data-prev]');
      if (prev) navigate(prev.href);
      break;
    }
    case ']': {
      event.preventDefault();
      const next = el<HTMLAnchorElement>('[data-next]');
      if (next) navigate(next.href);
      break;
    }
    case 'Escape':
      event.preventDefault();
      clearFind();
      state.findQuery = '';
      const input = el<HTMLInputElement>('[data-wr-find-input]');
      if (input) input.value = '';
      updateFooter();
      updateChip();
      break;
    default:
      break;
  }
}

// ---------------------------------------------------------------- panel filter

function openPanelFilter(): void {
  if (state.mode !== 'menu' && state.mode !== 'toc' && state.mode !== 'settings') return;
  const wrap = el<HTMLElement>(`[data-panel="${state.mode}"] [data-panel-filter-wrap]`);
  const input = el<HTMLInputElement>(`[data-panel="${state.mode}"] [data-panel-filter]`);
  if (!wrap || !input) return;
  wrap.hidden = false;
  state.filterOpen = true;
  input.value = '';
  state.filter = '';
  input.focus();
}

function hidePanelFilter(): void {
  els<HTMLElement>('[data-panel-filter-wrap]').forEach((wrap) => (wrap.hidden = true));
  els<HTMLInputElement>('[data-panel-filter]').forEach((input) => (input.value = ''));
}

function filterRows(mode: Mode): void {
  const panel = el<HTMLElement>(`[data-panel="${mode}"]`);
  if (!panel) return;
  const rows = els<HTMLElement>('.wr-row, .wr-settings-row', panel);
  const query = state.filter.toLowerCase();
  rows.forEach((row) => {
    row.hidden = Boolean(query) && !(row.textContent ?? '').toLowerCase().includes(query);
  });
  setSelectedIndex(mode, 0);
  selectRows(mode);
}

// ---------------------------------------------------------------- init

function bindOnce(): void {
  document.addEventListener('keydown', onKeyDown);
  document.addEventListener('scroll', scheduleChrome, true);
  window.addEventListener('resize', scheduleChrome);
  window.addEventListener('pagehide', saveScroll);

  document.addEventListener('click', (event) => {
    const target = event.target as HTMLElement | null;
    if (!target) return;
    if (target.closest('[data-wr-layer]') === target) {
      closePanel();
      return;
    }
    const setting = target.closest<HTMLElement>('[data-setting]');
    if (setting && setting.closest('.wr-panel')) {
      event.preventDefault();
      cycleSetting(setting.dataset.setting!);
      return;
    }
    const action = target.closest<HTMLElement>('[data-action]');
    if (action && action.closest('.wr-panel')) {
      event.preventDefault();
      activateRow(action);
      return;
    }
    const row = target.closest<HTMLElement>('.wr-row');
    if (row && row.closest('.wr-panel')) {
      const panel = row.closest<HTMLElement>('.wr-panel');
      const name = panel?.dataset.panel as Mode | undefined;
      if (name === 'menu' || name === 'toc' || name === 'settings') {
        const rows = panelRows(name);
        const index = rows.indexOf(row);
        if (index >= 0) {
          setSelectedIndex(name, index);
          selectRows(name);
        }
      }
    }
    if (target.closest('[data-wr-scroll]')) {
      const scroll = target.closest<HTMLElement>('[data-wr-scroll]')!;
      const body = bodyEl();
      if (!body) return;
      const rect = scroll.getBoundingClientRect();
      const ratio = clamp((event.clientY - rect.top) / rect.height, 0, 1);
      body.scrollTo({ top: ratio * (body.scrollHeight - body.clientHeight), behavior: 'auto' });
    }
  });

  // Panel filters.
  document.addEventListener('input', (event) => {
    const input = event.target as HTMLInputElement | null;
    if (!input?.matches('[data-panel-filter]')) return;
    state.filter = input.value;
    if (state.mode === 'menu' || state.mode === 'toc' || state.mode === 'settings') filterRows(state.mode);
  });
  document.addEventListener('keydown', (event) => {
    const input = event.target as HTMLInputElement | null;
    if (!input?.matches('[data-panel-filter]')) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      state.filterOpen = false;
      state.filter = '';
      input.value = '';
      hidePanelFilter();
      if (state.mode === 'menu' || state.mode === 'toc' || state.mode === 'settings') filterRows(state.mode);
    } else if (event.key === 'Enter') {
      event.preventDefault();
      event.stopPropagation();
      if (state.mode === 'menu' || state.mode === 'toc') chooseSelection(state.mode);
      else if (state.mode === 'settings') {
        const row = panelRows('settings')[state.settingsIndex];
        if (row?.dataset.setting) cycleSetting(row.dataset.setting);
      }
    }
  });

  // Find input.
  const findInput = () => el<HTMLInputElement>('[data-wr-find-input]');
  document.addEventListener('input', (event) => {
    if (!(event.target as HTMLElement | null)?.matches('[data-wr-find-input]')) return;
    state.findQuery = findInput()?.value ?? '';
    runFind();
  });
  document.addEventListener('keydown', (event) => {
    if (!(event.target as HTMLElement | null)?.matches('[data-wr-find-input]')) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      closeFind(true);
    } else if (event.key === 'Enter') {
      event.preventDefault();
      event.stopPropagation();
      if (!state.findMarks.length) toast('no results', true);
      state.mode = 'read';
      updateFooter();
      bodyEl()?.focus({ preventScroll: true });
    } else if (event.key === 'ArrowDown') {
      event.preventDefault();
      event.stopPropagation();
      focusMatch(state.findIndex + 1);
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      event.stopPropagation();
      focusMatch(state.findIndex - 1);
    }
  });

  // Omni input.
  document.addEventListener('input', (event) => {
    if ((event.target as HTMLElement | null)?.matches('[data-omni-input]')) updateOmni();
  });
  document.addEventListener('keydown', (event) => {
    if (!(event.target as HTMLElement | null)?.matches('[data-omni-input]')) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      closePanel();
    } else if (event.key === 'ArrowDown') {
      event.preventDefault();
      event.stopPropagation();
      moveOmni(1);
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      event.stopPropagation();
      moveOmni(-1);
    } else if (event.key === 'Enter') {
      event.preventDefault();
      event.stopPropagation();
      chooseOmni();
    }
  });
}

function saveScroll(): void {
  try {
    const map = JSON.parse(sessionStorage.getItem('wr-web-scroll') || '{}') as Record<string, number>;
    if (state.lastPath) map[state.lastPath] = state.lastScroll;
    sessionStorage.setItem('wr-web-scroll', JSON.stringify(map));
  } catch {
    /* ignore */
  }
}

function restoreScroll(): void {
  const body = bodyEl();
  if (window.location.hash) {
    if (body) {
      state.lastPath = window.location.pathname;
      state.lastScroll = body.scrollTop;
    }
    return;
  }
  try {
    const map = JSON.parse(sessionStorage.getItem('wr-web-scroll') || '{}') as Record<string, number>;
    const y = map[window.location.pathname];
    if (body && typeof y === 'number') body.scrollTop = y;
    state.lastPath = window.location.pathname;
    state.lastScroll = body?.scrollTop ?? 0;
  } catch {
    state.lastPath = window.location.pathname;
    state.lastScroll = 0;
  }
}

function refresh(): void {
  loadSettings();
  applySettings();
  updateFooter();
  restoreScroll();
  updateChrome();
  startSearchPage();
  // Restore the accent applied by the inline head script before the module ran.
  if (!bodyEl()) return;
}

export function initTui(): void {
  loadSettings();
  applySettings();
  if (!started) {
    started = true;
    bindOnce();
  }
  refresh();
}

initTui();
document.addEventListener('astro:page-load', () => {
  initTui();
});
document.addEventListener('astro:before-swap', () => {
  saveScroll();
  closePanel();
  clearFind();
  state.findQuery = '';
  state.mode = 'read';
});
