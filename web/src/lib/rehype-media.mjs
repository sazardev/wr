// Turn Markdown media into inert placeholders at build time.
//
// The server never emits an <img src> or a <video src> for page media. That is
// the whole point of "off by default": the browser cannot download a byte until
// the client decides to hydrate the placeholder with the Images/Videos setting.
// The placeholder still carries the URL, the alt/title and a real link, so the
// page is useful without JavaScript and the client only needs to swap in the
// player.

/** @param {unknown} node */
function isElement(node) {
  return Boolean(node && typeof node === 'object' && node.type === 'element');
}

/** @param {string} tag @param {Record<string, unknown>} properties @param {unknown[]} children */
function element(tag, properties, children) {
  return { type: 'element', tagName: tag, properties, children };
}

/** @param {string} value */
function text(value) {
  return { type: 'text', value };
}

/** @param {string} src @param {string} label */
function openLink(src, label) {
  return element('a', { href: src, target: '_blank', rel: ['noopener', 'noreferrer'] }, [
    text(`${label} ↗`),
  ]);
}

/** @param {any} node */
function mediaPlaceholder(node) {
  if (!isElement(node)) return null;
  const props = node.properties ?? {};

  if (node.tagName === 'img') {
    const src = typeof props.src === 'string' ? props.src : '';
    if (!src) return null;
    const alt = typeof props.alt === 'string' && props.alt.trim() ? props.alt.trim() : 'image';
    const width = props.width ? String(props.width) : '';
    const height = props.height ? String(props.height) : '';
    return element(
      'span',
      {
        className: ['wr-media', 'wr-media-image'],
        'data-media': 'image',
        'data-src': src,
        'data-alt': alt,
        'data-width': width || undefined,
        'data-height': height || undefined,
      },
      [
        element('span', { className: ['wr-media-off'] }, [
          text(`[image: ${alt}] `),
          openLink(src, 'open'),
        ]),
      ],
    );
  }

  if (node.tagName === 'video') {
    const sources = Array.isArray(node.children) ? node.children : [];
    const source = sources.find((child) => isElement(child) && child.tagName === 'source');
    const src =
      (typeof props.src === 'string' && props.src) ||
      (source && typeof source.properties?.src === 'string' ? source.properties.src : '');
    if (!src) return null;
    const title =
      (typeof props.title === 'string' && props.title.trim()) ||
      (typeof props.alt === 'string' && props.alt.trim()) ||
      'video';
    const poster = typeof props.poster === 'string' ? props.poster : '';
    const width = props.width ? String(props.width) : '';
    const height = props.height ? String(props.height) : '';
    return element(
      'div',
      {
        className: ['wr-media', 'wr-media-video'],
        'data-media': 'video',
        'data-src': src,
        'data-poster': poster,
        'data-label': title,
        'data-width': width || undefined,
        'data-height': height || undefined,
      },
      [
        element('span', { className: ['wr-media-off'] }, [
          text(`[video: ${title}] `),
          openLink(src, 'open'),
        ]),
      ],
    );
  }

  return null;
}

/** @param {any} node */
function walk(node) {
  if (!node || typeof node !== 'object') return;
  if (!Array.isArray(node.children)) return;
  for (let i = 0; i < node.children.length; i++) {
    const child = node.children[i];
    const replacement = mediaPlaceholder(child);
    if (replacement) {
      node.children[i] = replacement;
      continue;
    }
    walk(child);
  }
}

export function rehypeMedia() {
  return (tree) => walk(tree);
}
