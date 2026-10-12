// The composition root of the client side. Each feature is a module with one
// job that activates itself when the markup it belongs to is present; this
// file only decides when to run them — once for the global listeners, and again
// after every view-transition navigation (Astro fires astro:page-load), when
// the page's elements are new.

import { startCopy } from './copy';
import { startFilter } from './filter';
import { startHelp } from './help';
import { startHotkeys } from './hotkeys';
import { startInstall } from './install';
import { startNav } from './nav';
import { startNoise } from './noise';
import { startPalette } from './palette';
import { startReel } from './reel';
import { startReveal } from './reveal';
import { startProse } from './code';
import { startScrollspy } from './scrollspy';
import { startSearch } from './search';
import { startStatus } from './status';
import { startThemeMenu, startThemes } from './themes';

function platformKeys(): void {
  const mac = /Mac|iPhone|iPad/.test(navigator.userAgent);
  document.querySelectorAll<HTMLElement>('[data-mod]').forEach((el) => (el.textContent = mac ? '⌘' : 'Ctrl'));
}

export function enhance(): void {
  startHotkeys();
  startCopy();
  startNav();
  startThemes();
  startThemeMenu();
  startPalette();
  startHelp();
  startStatus();
  startProse();
  startScrollspy();
  startInstall();
  startReel();
  startNoise();
  startFilter();
  void startSearch();
  startReveal();
  platformKeys();
}

enhance();
document.addEventListener('astro:page-load', enhance);
