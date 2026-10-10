// The composition root of the client side. Each feature is a module with one
// job that activates itself when the page it belongs to is present: this file
// only decides when to run them — once per document for the global chrome, and
// again after a view-transition navigation (Astro fires astro:page-load).

import { startCopy } from './copy';
import { startCursor } from './cursor';
import { startField } from './field';
import { startHotkeys } from './hotkeys';
import { startProgress } from './progress';
import { startReel } from './reel';
import { fillRules } from './rules';
import { startSearch } from './search';
import { startSpin } from './spin';
import { startThemes } from './themes';

let chromeReady = false;

export function enhance(): void {
  if (!chromeReady) {
    chromeReady = true;
    startThemes();
    startField();
    startCursor();
    startHotkeys();
    startProgress();
  }
  // per page: the reel, the search results and the copy buttons belong to the
  // page's markup and are wired again after a view-transition navigation
  startReel();
  void startSearch();
  startSpin();
  startCopy();
  fillRules();
}

enhance();
document.addEventListener('astro:page-load', enhance);
addEventListener('resize', () => fillRules());
if (document.fonts?.ready) document.fonts.ready.then(() => fillRules());
