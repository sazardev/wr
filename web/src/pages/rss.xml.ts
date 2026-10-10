import rss from '@astrojs/rss';
import { getCollection } from 'astro:content';
import { parseChangelog } from '../lib/changelog';
import { readChangelog } from '../lib/repo';
import { withBase } from '../lib/urls';

// A release feed, generated from CHANGELOG.md: the changelog release-please
// writes is the feed, so subscribing to the docs follows the releases.

export async function GET(context: { site: URL }) {
  const changelog = parseChangelog(readChangelog());
  const entries = await getCollection('docs');
  const release = (version: string) => entries.find((entry) => entry.data.section === `v${version}`);

  return rss({
    title: 'wr releases',
    description: 'Every release of wr, written by release-please from the repository history.',
    site: context.site,
    trailingSlash: true,
    items: changelog.releases
      .map((item) => {
        const page = release(item.version);
        return {
          title: `wr ${item.version}`,
          link: page ? withBase(`/docs/changelog/v${item.version}/`) : undefined,
          pubDate: item.date ? new Date(item.date) : new Date(0),
          description: item.text,
        };
      })
      .filter((item) => item.link),
    customData: `<language>en-us</language>`,
  });
}
