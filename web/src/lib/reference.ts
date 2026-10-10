import { groupBindings, parseKeybindings, type KeyBinding } from './keybindings';
import { cellText, tableUnderHeading } from './tables';
import { readKeysGo, readReadme } from './repo';

// Reference data, all of it generated from the repository: the settings and
// paths tables of the README and the key bindings of keys.go. Nothing here is
// typed by hand, so the reference cannot fall behind the program.

export interface SettingRow {
  key: string;
  default: string;
  description: string;
}

export interface PathRow {
  path: string;
  what: string;
}

export interface ShortcutGroup {
  group: string;
  items: KeyBinding[];
}

/** The Configuration table of the README, as settings. */
export function settingRows(): SettingRow[] {
  const table = tableUnderHeading(readReadme(), 'Configuration');
  if (!table) return [];
  return table.rows
    .filter((row) => row.length >= 3)
    .map((row) => ({ key: cellText(row[0]!), default: cellText(row[1]!), description: row[2]! }));
}

/** The "Where things live" table of the README. */
export function pathRows(): PathRow[] {
  const table = tableUnderHeading(readReadme(), 'Where things live');
  if (!table) return [];
  return table.rows.filter((row) => row.length >= 2).map((row) => ({ path: cellText(row[0]!), what: row[1]! }));
}

/** The default key bindings, grouped as the program's shortcuts panel shows them. */
export function shortcutGroups(): ShortcutGroup[] {
  return groupBindings(parseKeybindings(readKeysGo()).bindings);
}

/** The bindings the reader shows in its own footer, in the footer's order. */
export const FOOTER_ACTIONS = ['open', 'back', 'forward', 'follow', 'index', 'reload', 'menu', 'quit'] as const;

/** A subset of the shortcuts panel, enough for the landing page. */
export const TEASER_GROUPS = ['Browsing', 'Scrolling', 'Reading', 'Interface'] as const;

function bindingMap(): Map<string, KeyBinding> {
  return new Map(parseKeybindings(readKeysGo()).bindings.map((binding) => [binding.id, binding]));
}

export function footerBindings(): KeyBinding[] {
  const map = bindingMap();
  return FOOTER_ACTIONS.map((id) => map.get(id)).filter((binding): binding is KeyBinding => Boolean(binding));
}

export function teaserBindings(): KeyBinding[] {
  const groups = shortcutGroups();
  const wanted = new Set<string>(TEASER_GROUPS);
  return groups.filter((group) => wanted.has(group.group)).flatMap((group) => group.items);
}
