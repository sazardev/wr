// The recorded scenes: the same frames scripts/capture.sh regenerates from the
// real program. They are data, so the landing page, the guide and the gallery
// all show the same list in the same order.

export interface Scene {
  name: string;
  /** The command or key that produced the frame. */
  cmd: string;
  title: string;
  /** Alt text: what the frame shows, for people who cannot see it. */
  alt: string;
}

export const SCENES: readonly Scene[] = [
  {
    name: 'ui',
    cmd: 'wr https://example.com/article',
    title: 'reading',
    alt: 'wr open on an article: the reading view with a code block and the footer shortcuts',
  },
  {
    name: 'links',
    cmd: 'f',
    title: 'links',
    alt: 'wr with every link labelled with a short hint',
  },
  {
    name: 'sections',
    cmd: 't',
    title: 'sections',
    alt: 'the section index, listing the headings of the page',
  },
  {
    name: 'shortcuts',
    cmd: 'm',
    title: 'menu',
    alt: 'the menu, with every shortcut of the program',
  },
  {
    name: 'search',
    cmd: '/ braille',
    title: 'search',
    alt: 'search as you type, with the matches highlighted',
  },
];

export const sceneAsset = (scene: Scene): string => `/assets/scenes/${scene.name}.svg`;
