import { describe, expect, it } from 'vitest';
import { rehypeMedia } from './rehype-media.mjs';

function treeWith(...children: unknown[]) {
  return { type: 'root', children };
}

describe('rehypeMedia', () => {
  it('turns images into inert placeholders with no img to fetch', () => {
    const tree: any = treeWith({
      type: 'element',
      tagName: 'p',
      children: [
        {
          type: 'element',
          tagName: 'img',
          properties: { src: '/media/a.png', alt: 'A diagram', width: 1200, height: 800 },
          children: [],
        },
      ],
    });
    rehypeMedia()(tree);
    const json = JSON.stringify(tree);
    expect(json).toContain('wr-media-image');
    expect(json).toContain('data-src');
    expect(json).toContain('/media/a.png');
    expect(json).toContain('A diagram');
    expect(json).toContain('1200');
    expect(json).not.toContain('"tagName":"img"');
  });

  it('turns a video (and its source) into a placeholder with no video to fetch', () => {
    const tree: any = treeWith({
      type: 'element',
      tagName: 'div',
      children: [
        {
          type: 'element',
          tagName: 'video',
          properties: { poster: '/media/p.png', title: 'The reader' },
          children: [{ type: 'element', tagName: 'source', properties: { src: '/media/v.webm' }, children: [] }],
        },
      ],
    });
    rehypeMedia()(tree);
    const json = JSON.stringify(tree);
    expect(json).toContain('wr-media-video');
    expect(json).toContain('/media/v.webm');
    expect(json).toContain('/media/p.png');
    expect(json).toContain('The reader');
    expect(json).not.toContain('"tagName":"video"');
  });

  it('leaves normal content alone', () => {
    const tree: any = treeWith({
      type: 'element',
      tagName: 'p',
      children: [{ type: 'text', value: 'no media here' }],
    });
    rehypeMedia()(tree);
    expect(JSON.stringify(tree)).toContain('no media here');
  });
});
