package main

import (
	"strings"
)

// Key bindings are data: every reading-mode action has default keys, and the
// user's overrides (from config.toml or the Keyboard shortcuts panel) replace
// them per action. Keys are the names Bubble Tea reports: "j", "G", "down",
// "ctrl+f", "alt+left", "shift+tab", "space", "enter", ...

type action string

const (
	actOpen      action = "open"
	actBack      action = "back"
	actForward   action = "forward"
	actReload    action = "reload"
	actHome      action = "home"
	actHistory   action = "history"
	actBookmarks action = "bookmarks"
	actBookmark  action = "bookmark"
	actExternal  action = "external"
	actDown      action = "down"
	actUp        action = "up"
	actPageDown  action = "page_down"
	actPageUp    action = "page_up"
	actHalfDown  action = "half_down"
	actHalfUp    action = "half_up"
	actTop       action = "top"
	actBottom    action = "bottom"
	actNextSec   action = "next_section"
	actPrevSec   action = "prev_section"
	actIndex     action = "index"
	actSearch    action = "search"
	actNextMatch action = "next_match"
	actPrevMatch action = "prev_match"
	actFollow    action = "follow"
	actNextLink  action = "next_link"
	actPrevLink  action = "prev_link"
	actActivate  action = "activate"
	actYank      action = "yank"
	actMenu      action = "menu"
	actSettings  action = "settings"
	actQuit      action = "quit"
)

type actionInfo struct {
	id    action
	group string
	label string
}

// actionList is the display order of the Keyboard shortcuts panel.
var actionList = []actionInfo{
	{actOpen, "Browsing", "Open URL or search"},
	{actBack, "Browsing", "Back"},
	{actForward, "Browsing", "Forward"},
	{actReload, "Browsing", "Reload page"},
	{actHome, "Browsing", "Start page"},
	{actHistory, "Browsing", "History"},
	{actBookmarks, "Browsing", "Bookmarks"},
	{actBookmark, "Browsing", "Bookmark this page"},
	{actExternal, "Browsing", "Open in your browser"},
	{actFollow, "Links", "Follow a link (hint labels)"},
	{actNextLink, "Links", "Next link"},
	{actPrevLink, "Links", "Previous link"},
	{actActivate, "Links", "Open the focused link"},
	{actDown, "Scrolling", "Line down"},
	{actUp, "Scrolling", "Line up"},
	{actPageDown, "Scrolling", "Page down"},
	{actPageUp, "Scrolling", "Page up"},
	{actHalfDown, "Scrolling", "Half page down"},
	{actHalfUp, "Scrolling", "Half page up"},
	{actTop, "Scrolling", "Top"},
	{actBottom, "Scrolling", "Bottom"},
	{actNextSec, "Reading", "Next section"},
	{actPrevSec, "Reading", "Previous section"},
	{actIndex, "Reading", "Section index"},
	{actSearch, "Reading", "Search in page"},
	{actNextMatch, "Reading", "Next match"},
	{actPrevMatch, "Reading", "Previous match"},
	{actYank, "Reading", "Copy the selection"},
	{actMenu, "Interface", "Menu"},
	{actSettings, "Interface", "Settings"},
	{actQuit, "Interface", "Quit"},
}

var defaultKeys = map[action][]string{
	actOpen:      {"o"},
	actBack:      {"H", "backspace", "alt+left"},
	actForward:   {"L", "alt+right"},
	actReload:    {"r"},
	actHome:      {"~"},
	actHistory:   {"v"},
	actBookmarks: {"'"},
	actBookmark:  {"*"},
	actExternal:  {"x"},
	actDown:      {"j", "down"},
	actUp:        {"k", "up"},
	actPageDown:  {"space", "pgdown", "ctrl+f"},
	actPageUp:    {"b", "pgup", "ctrl+b"},
	actHalfDown:  {"d", "ctrl+d"},
	actHalfUp:    {"u", "ctrl+u"},
	actTop:       {"g", "home"},
	actBottom:    {"G", "end"},
	actNextSec:   {"]"},
	actPrevSec:   {"["},
	actIndex:     {"t"},
	actSearch:    {"/"},
	actNextMatch: {"n"},
	actPrevMatch: {"N"},
	actFollow:    {"f"},
	actNextLink:  {"tab"},
	actPrevLink:  {"shift+tab"},
	actActivate:  {"enter"},
	actYank:      {"y"},
	actMenu:      {"m", "?"},
	actSettings:  {","},
	actQuit:      {"q"},
}

func sameKeys(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// effectiveKeys are the keys bound to an action: the user's override if there
// is one (an empty list means "unbound"), else the defaults.
func effectiveKeys(c *Config, a action) []string {
	if k, ok := c.Keys[string(a)]; ok {
		return k
	}
	return defaultKeys[a]
}

// buildKeymap maps each key to its action. If two actions claim a key (a
// hand-edited file), the one listed first in actionList wins.
func buildKeymap(c *Config) map[string]action {
	km := map[string]action{}
	for i := len(actionList) - 1; i >= 0; i-- {
		a := actionList[i].id
		for _, k := range effectiveKeys(c, a) {
			km[k] = a
		}
	}
	return km
}

// setKeys binds keys to an action, taking them away from any other action that
// had them, and drops overrides that equal the defaults.
func setKeys(c *Config, a action, keys []string) {
	if c.Keys == nil {
		c.Keys = map[string][]string{}
	}
	taken := map[string]bool{}
	for _, k := range keys {
		taken[k] = true
	}
	for _, info := range actionList {
		if info.id == a {
			continue
		}
		cur := effectiveKeys(c, info.id)
		var kept []string
		for _, k := range cur {
			if !taken[k] {
				kept = append(kept, k)
			}
		}
		if len(kept) != len(cur) {
			c.Keys[string(info.id)] = append([]string{}, kept...)
		}
	}
	c.Keys[string(a)] = append([]string{}, keys...)
	for id, v := range c.Keys {
		if sameKeys(v, defaultKeys[action(id)]) {
			delete(c.Keys, id)
		}
	}
	if len(c.Keys) == 0 {
		c.Keys = nil
	}
}

func resetKeys(c *Config) { c.Keys = nil }

var keyNames = map[string]string{
	"down": "↓", "up": "↑", "left": "←", "right": "→", "enter": "↵", "backspace": "⌫",
	"pgdown": "PgDn", "pgup": "PgUp", "shift+tab": "⇧tab", "alt+left": "⌥←", "alt+right": "⌥→",
}

// prettyKey is how a key is shown to people.
func prettyKey(k string) string {
	if n, ok := keyNames[k]; ok {
		return n
	}
	if strings.HasPrefix(k, "ctrl+") {
		return "^" + strings.ToUpper(strings.TrimPrefix(k, "ctrl+"))
	}
	return k
}

// keyHint is the first key of an action, for the footer and menus.
func keyHint(c *Config, a action) string {
	ks := effectiveKeys(c, a)
	if len(ks) == 0 {
		return ""
	}
	return prettyKey(ks[0])
}

// keyList is every key of an action, for the shortcuts panel.
func keyList(c *Config, a action) string {
	ks := effectiveKeys(c, a)
	if len(ks) == 0 {
		return "unbound"
	}
	out := make([]string, len(ks))
	for i, k := range ks {
		out[i] = prettyKey(k)
	}
	return strings.Join(out, "  ")
}
