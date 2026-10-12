package main

// State tools: the reader's own store (history, bookmarks), the page cache and
// the configuration file. They read and write the same paths and formats the
// app uses, so an agent manages the store exactly like the UI would.

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

func registerStateTools(s *mcpServer) {
	tools := []mcpTool{
		{
			def: toolDef{
				Name:        "wr_history",
				Description: "The browsing history, most recent first, one entry per page. Optional filter matches url or title (case-insensitive).",
				InputSchema: objectSchema(jsonSchema{
					"limit":  intProp("maximum entries (default 50)"),
					"filter": strProp("only entries whose url or title contains this text"),
				}),
			},
			call: toolHistory,
		},
		{
			def: toolDef{
				Name:        "wr_bookmarks",
				Description: "The bookmarks, newest first.",
				InputSchema: objectSchema(jsonSchema{}),
			},
			call: func(s *mcpServer, raw json.RawMessage) (any, error) {
				vs := bookmarkList()
				return map[string]any{"count": len(vs), "bookmarks": visitList(vs)}, nil
			},
		},
		{
			def: toolDef{
				Name: "wr_bookmark_toggle",
				Description: "Add a page to the bookmarks, or remove it if it was already there. The url can be raw: " +
					"it is resolved with the same rules as the address bar.",
				InputSchema: objectSchema(jsonSchema{
					"target": strProp("an URL or file path"),
					"title":  strProp("the title to save with the bookmark (optional)"),
				}, "target"),
			},
			call: func(s *mcpServer, raw json.RawMessage) (any, error) {
				var a struct {
					Target string `json:"target"`
					Title  string `json:"title"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return nil, argError("bad arguments: " + err.Error())
				}
				if strings.TrimSpace(a.Target) == "" {
					return nil, argError("target is required")
				}
				cfg, _ := loadConfig()
				url, _ := normalizeInput(a.Target, cfg.SearchEngine)
				if url == "" {
					return nil, argError("empty target")
				}
				added, err := bookmarkToggle(url, a.Title)
				if err != nil {
					return nil, err
				}
				return map[string]any{"url": url, "bookmarked": added}, nil
			},
		},
		{
			def: toolDef{
				Name:        "wr_bookmark_delete",
				Description: "Remove one bookmark by url (exact match after address-bar resolution).",
				InputSchema: objectSchema(jsonSchema{
					"target": strProp("the url to remove"),
				}, "target"),
			},
			call: func(s *mcpServer, raw json.RawMessage) (any, error) {
				var a struct {
					Target string `json:"target"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return nil, argError("bad arguments: " + err.Error())
				}
				cfg, _ := loadConfig()
				url, _ := normalizeInput(a.Target, cfg.SearchEngine)
				if err := bookmarkDelete(url); err != nil {
					return nil, err
				}
				return map[string]any{"removed": url}, nil
			},
		},
		{
			def: toolDef{
				Name:        "wr_history_delete",
				Description: "Remove every history entry for one url (exact match after address-bar resolution).",
				InputSchema: objectSchema(jsonSchema{
					"target": strProp("the url to remove"),
				}, "target"),
			},
			call: func(s *mcpServer, raw json.RawMessage) (any, error) {
				var a struct {
					Target string `json:"target"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return nil, argError("bad arguments: " + err.Error())
				}
				cfg, _ := loadConfig()
				url, _ := normalizeInput(a.Target, cfg.SearchEngine)
				if err := historyDelete(url); err != nil {
					return nil, err
				}
				return map[string]any{"removed": url}, nil
			},
		},
		{
			def: toolDef{
				Name:        "wr_history_clear",
				Description: "Delete the whole history file (~/.local/state/wr/history.jsonl).",
				InputSchema: objectSchema(jsonSchema{}),
			},
			call: func(s *mcpServer, raw json.RawMessage) (any, error) {
				if err := historyClear(); err != nil {
					return nil, err
				}
				return map[string]any{"cleared": true}, nil
			},
		},
		{
			def: toolDef{
				Name:        "wr_cache_stats",
				Description: "How many pages are cached and how much disk they use.",
				InputSchema: objectSchema(jsonSchema{}),
			},
			call: func(s *mcpServer, raw json.RawMessage) (any, error) {
				pages, bytes := cacheStats()
				return map[string]any{"pages": pages, "bytes": bytes, "path": cacheDirForReport()}, nil
			},
		},
		{
			def: toolDef{
				Name:        "wr_cache_clear",
				Description: "Delete the page cache (~/.cache/wr).",
				InputSchema: objectSchema(jsonSchema{}),
			},
			call: func(s *mcpServer, raw json.RawMessage) (any, error) {
				pages, _ := cacheStats()
				if err := cacheClear(); err != nil {
					return nil, err
				}
				return map[string]any{"cleared_pages": pages}, nil
			},
		},
		{
			def: toolDef{
				Name:        "wr_config_get",
				Description: "The effective configuration (the file merged over the defaults) and the path of the config file.",
				InputSchema: objectSchema(jsonSchema{}),
			},
			call: func(s *mcpServer, raw json.RawMessage) (any, error) {
				cfg, cfgErr := loadConfig()
				path, _ := configPath()
				return map[string]any{
					"path":   path,
					"exists": cfgErr == nil,
					"config": configMap(&cfg),
				}, nil
			},
		},
		{
			def: toolDef{
				Name: "wr_config_set",
				Description: "Change settings and save them to the config file. Keys are the settings names " +
					"(width, center, spacing, links, accent, search_engine, history_limit, cache_days, ...). " +
					"Values are clamped with the same rules as the Settings panel.",
				InputSchema: objectSchema(jsonSchema{
					"settings": map[string]any{"type": "object", "additionalProperties": true,
						"description": "setting name -> new value (boolean, number or string)"},
				}, "settings"),
			},
			call: toolConfigSet,
		},
	}
	for _, t := range tools {
		s.register(t)
	}
}

func toolHistory(s *mcpServer, raw json.RawMessage) (any, error) {
	var a struct {
		Limit  int    `json:"limit"`
		Filter string `json:"filter"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, argError("bad arguments: " + err.Error())
	}
	if a.Limit <= 0 {
		a.Limit = 50
	}
	filter := strings.ToLower(strings.TrimSpace(a.Filter))
	var out []map[string]any
	for i, v := range historyList() {
		if i >= a.Limit {
			break
		}
		if filter != "" && !strings.Contains(strings.ToLower(v.URL), filter) && !strings.Contains(strings.ToLower(v.Title), filter) {
			continue
		}
		out = append(out, map[string]any{"url": v.URL, "title": v.Title, "at": v.At})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return map[string]any{"count": len(out), "history": out}, nil
}

func toolConfigSet(s *mcpServer, raw json.RawMessage) (any, error) {
	var a struct {
		Settings map[string]any `json:"settings"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, argError("bad arguments: " + err.Error())
	}
	if len(a.Settings) == 0 {
		return nil, argError("settings is required")
	}
	cfg, _ := loadConfig()
	if err := applyConfigSettings(&cfg, a.Settings); err != nil {
		return nil, argError(err.Error())
	}
	cfg.normalize()
	if err := saveConfig(cfg); err != nil {
		return nil, err
	}
	return map[string]any{"saved": true, "config": configMap(&cfg)}, nil
}

func visitList(vs []Visit) []map[string]any {
	out := make([]map[string]any, 0, len(vs))
	for _, v := range vs {
		out = append(out, map[string]any{"url": v.URL, "title": v.Title, "at": v.At})
	}
	return out
}

// configMap is the Config as a map keyed by its settings names (the toml tags),
// so agents see the same names the Settings panel uses.
func configMap(c *Config) map[string]any {
	out := map[string]any{}
	v := reflect.ValueOf(c).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("toml")
		if tag == "" || tag == "-" {
			continue
		}
		f := v.Field(i)
		if f.Kind() == reflect.Map {
			m := map[string]any{}
			for _, k := range f.MapKeys() {
				m[fmt.Sprint(k.Interface())] = f.MapIndex(k).Interface()
			}
			out[tag] = m
			continue
		}
		out[tag] = f.Interface()
	}
	return out
}

// applyConfigSettings assigns a settings map onto a Config by settings name.
func applyConfigSettings(c *Config, settings map[string]any) error {
	v := reflect.ValueOf(c).Elem()
	t := v.Type()
	for key, val := range settings {
		idx := -1
		for i := 0; i < t.NumField(); i++ {
			if t.Field(i).Tag.Get("toml") == key {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("unknown setting: %s", key)
		}
		f := v.Field(idx)
		switch f.Kind() {
		case reflect.Bool:
			b, ok := val.(bool)
			if !ok {
				return fmt.Errorf("%s: expected true or false", key)
			}
			f.SetBool(b)
		case reflect.Int, reflect.Int64:
			n, ok := val.(float64)
			if !ok {
				return fmt.Errorf("%s: expected a number", key)
			}
			f.SetInt(int64(n))
		case reflect.String:
			s, ok := val.(string)
			if !ok {
				return fmt.Errorf("%s: expected a string", key)
			}
			f.SetString(s)
		case reflect.Map:
			m, ok := val.(map[string]any)
			if !ok {
				return fmt.Errorf("%s: expected an object of action -> keys", key)
			}
			keys := map[string][]string{}
			for action, ks := range m {
				list, ok := ks.([]any)
				if !ok {
					return fmt.Errorf("%s[%s]: expected a list of key names", key, action)
				}
				for _, k := range list {
					s, ok := k.(string)
					if !ok {
						return fmt.Errorf("%s[%s]: keys must be strings", key, action)
					}
					keys[action] = append(keys[action], s)
				}
			}
			f.Set(reflect.ValueOf(keys))
		default:
			return fmt.Errorf("%s: cannot be set", key)
		}
	}
	return nil
}

func cacheDirForReport() string {
	d, err := cacheDir()
	if err != nil {
		return ""
	}
	return d
}
