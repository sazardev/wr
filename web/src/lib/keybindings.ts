// The shortcut reference is generated from keys.go: the file that decides what
// the program actually binds. One parser, one test, and the reference page can
// never list a key the program does not have.

export interface KeyBinding {
  /** Action name, lowercased with underscores: `next_section`. */
  id: string;
  /** Group in the program's shortcuts panel: Browsing, Links, … */
  group: string;
  /** Human label of the action. */
  label: string;
  /** Default keys, as Bubble Tea reports them. */
  keys: string[];
}

export interface Keybindings {
  groups: string[];
  bindings: KeyBinding[];
}

const ACTION = /\{\s*act(\w+)\s*,\s*"([^"]*)"\s*,\s*"([^"]*)"\s*\}/g;
const DEFAULTS = /act(\w+)\s*:\s*\{([^}]*)\}/g;

/** Parses the `actionList` and `defaultKeys` tables of keys.go. */
export function parseKeybindings(goSource: string): Keybindings {
  const keyMap = new Map<string, string[]>();
  for (const match of goSource.matchAll(DEFAULTS)) {
    const name = match[1]!;
    const keys = [...match[2]!.matchAll(/"([^"]*)"/g)].map((k) => k[1]!);
    keyMap.set(lowerSnake(name), keys);
  }

  const bindings: KeyBinding[] = [];
  for (const match of goSource.matchAll(ACTION)) {
    bindings.push({
      id: lowerSnake(match[1]!),
      group: match[2]!,
      label: match[3]!,
      keys: keyMap.get(lowerSnake(match[1]!)) ?? [],
    });
  }

  const groups: string[] = [];
  for (const binding of bindings) {
    if (!groups.includes(binding.group)) groups.push(binding.group);
  }
  return { groups, bindings };
}

/** actNextSection -> next_section */
function lowerSnake(camel: string): string {
  return camel
    .replace(/([a-z0-9])([A-Z])/g, '$1_$2')
    .replace(/([A-Z]+)([A-Z][a-z])/g, '$1_$2')
    .toLowerCase();
}

/** Actions grouped, keeping the program's order inside each group. */
export function groupBindings(bindings: KeyBinding[]): { group: string; items: KeyBinding[] }[] {
  const groups: { group: string; items: KeyBinding[] }[] = [];
  for (const binding of bindings) {
    let bucket = groups.find((g) => g.group === binding.group);
    if (!bucket) {
      bucket = { group: binding.group, items: [] };
      groups.push(bucket);
    }
    bucket.items.push(binding);
  }
  return groups;
}
