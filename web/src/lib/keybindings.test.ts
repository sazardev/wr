import { describe, expect, it } from 'vitest';
import { groupBindings, parseKeybindings } from './keybindings';

const keysGo = `package main

var actionList = []actionInfo{
	{actOpen, "Browsing", "Open URL or search"},
	{actBack, "Browsing", "Back"},
	{actForward, "Browsing", "Forward"},
	{actFollow, "Links", "Follow a link (hint labels)"},
	{actNextSec, "Reading", "Next section"},
	{actMenu, "Interface", "Menu"},
}

var defaultKeys = map[action][]string{
	actOpen:      {"o"},
	actBack:      {"H", "backspace", "alt+left"},
	actForward:   {"L", "alt+right"},
	actFollow:    {"f"},
	actNextSec:   {"]"},
	actMenu:      {"m", "?"},
}
`;

describe('parseKeybindings', () => {
  const { bindings, groups } = parseKeybindings(keysGo);

  it('reads every action with its group and label', () => {
    expect(bindings.map((b) => b.id)).toEqual(['open', 'back', 'forward', 'follow', 'next_sec', 'menu']);
    expect(bindings[1]).toMatchObject({ group: 'Browsing', label: 'Back', keys: ['H', 'backspace', 'alt+left'] });
  });

  it('keeps the display order of the actions', () => {
    expect(groups).toEqual(['Browsing', 'Links', 'Reading', 'Interface']);
  });

  it('groups actions without reordering them', () => {
    const grouped = groupBindings(bindings);
    expect(grouped.map((g) => g.group)).toEqual(['Browsing', 'Links', 'Reading', 'Interface']);
    expect(grouped[0]!.items.map((b) => b.id)).toEqual(['open', 'back', 'forward']);
  });

  it('reports actions that have no default keys', () => {
    const rebound = parseKeybindings('var actionList = []actionInfo{{actQuit, "Interface", "Quit"}}');
    expect(rebound.bindings[0]!.keys).toEqual([]);
  });
});
