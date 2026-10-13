// IndexNow: tells Bing, Yandex, Seznam, Naver and Yep that the site changed,
// without waiting for a crawl. The key file it verifies against lives in
// public/ and is served from the site itself.
//
// Usage: node scripts/indexnow.mjs [site-url]
//   site-url defaults to SITE_DEPLOY_URL, then to the GitHub Pages project URL.

import { readFileSync, readdirSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const publicDir = path.resolve(here, '../public');

const raw = process.argv[2] ?? process.env.SITE_DEPLOY_URL ?? 'https://sazardev.github.io/wr/';
const site = new URL(raw.endsWith('/') ? raw : `${raw}/`);
const basePath = site.pathname.replace(/\/$/, '');

const keyFile = readdirSync(publicDir).find((name) => /^[a-f0-9]{32}\.txt$/.test(name));
if (!keyFile) {
  console.error('indexnow: no {key}.txt file in public/');
  process.exit(1);
}
const key = readFileSync(path.join(publicDir, keyFile), 'utf8').trim();

const absolutize = (pathname) => new URL(`${basePath}${pathname}`, site.origin).toString();

async function sitemapUrls() {
  for (const name of ['/sitemap-0.xml', '/sitemap-index.xml']) {
    try {
      const xml = await (await fetch(absolutize(name))).text();
      const locs = [...xml.matchAll(/<loc>([^<]+)<\/loc>/g)].map((match) => match[1].trim());
      const pages = locs.filter((loc) => loc.endsWith('.xml') === false && loc.endsWith('.txt') === false);
      if (pages.length) return pages;
    } catch (error) {
      console.warn(`indexnow: ${name} not reachable (${error.message})`);
    }
  }
  return [site.toString()];
}

const urlList = await sitemapUrls();
const payload = {
  host: site.host,
  key,
  keyLocation: absolutize(`/${keyFile}`),
  urlList,
};

// The key file may not be live yet right after a deploy: retry a few times.
for (let attempt = 1; attempt <= 5; attempt++) {
  try {
    const response = await fetch('https://api.indexnow.org/indexnow', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json; charset=utf-8' },
      body: JSON.stringify(payload),
    });
    if (response.ok || response.status === 202) {
      console.log(`indexnow: submitted ${urlList.length} URLs (${response.status})`);
      process.exit(0);
    }
    console.warn(`indexnow: attempt ${attempt} returned ${response.status}`);
  } catch (error) {
    console.warn(`indexnow: attempt ${attempt} failed (${error.message})`);
  }
  await new Promise((resolve) => setTimeout(resolve, attempt * 5000));
}

console.warn('indexnow: gave up; the sitemap in robots.txt remains the fallback');
