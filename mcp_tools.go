package main

import (
	"encoding/json"
	"net/url"
	"strings"
)

// Reading tools: they wrap the same pipeline the pipe mode of the CLI uses
// (normalizeInput -> load -> renderDoc), so what an agent reads here is
// exactly what the binary would show.

// toolNames is the order tools are listed in: reading, then sessions, then the
// local store.
var toolNames = []string{
	"wr_read", "wr_render", "wr_links", "wr_resolve_input",
	"wr_session_open", "wr_session_key", "wr_session_type", "wr_session_screen", "wr_session_close", "wr_session_list",
	"wr_history", "wr_bookmarks", "wr_bookmark_toggle", "wr_bookmark_delete", "wr_history_delete", "wr_history_clear",
	"wr_cache_stats", "wr_cache_clear", "wr_config_get", "wr_config_set",
}

func registerTools(s *mcpServer) {
	read := []mcpTool{
		{
			def: toolDef{
				Name: "wr_read",
				Description: "Fetch a page, file or search words and return the extracted article as Markdown. " +
					"A target is an URL (http/https/file), a local path, or plain words (searched with the configured engine). Always fetches fresh.",
				InputSchema: objectSchema(jsonSchema{
					"target":      strProp("an URL, a local file path, or search words"),
					"strip_links": boolProp("drop the URLs from Markdown links, keeping their text (default false)"),
				}, "target"),
			},
			call: toolRead,
		},
		{
			def: toolDef{
				Name: "wr_render",
				Description: "Fetch a page and render it the way the reader shows it: plain text by default (ANSI off), " +
					"one string per screen line, wrapped to width. Use ansi=true to keep the colors.",
				InputSchema: objectSchema(jsonSchema{
					"target": strProp("an URL, a local file path, or search words"),
					"width":  intProp("column width (default: the configured width, else 100)"),
					"ansi":   boolProp("keep the ANSI colors in the output (default false)"),
				}, "target"),
			},
			call: toolRender,
		},
		{
			def: toolDef{
				Name: "wr_links",
				Description: "Fetch a page and list its links in reading order: the link text, the destination as written, " +
					"and the absolute URL it opens to (anchors and unsupported schemes like mailto: included).",
				InputSchema: objectSchema(jsonSchema{
					"target": strProp("an URL, a local file path, or search words"),
				}, "target"),
			},
			call: toolLinks,
		},
		{
			def: toolDef{
				Name:        "wr_resolve_input",
				Description: "Say what wr would do with an address-bar entry (an URL, a file, or a web search) without fetching anything.",
				InputSchema: objectSchema(jsonSchema{
					"input":  strProp("the raw address-bar text"),
					"engine": strProp("search engine preset or template with %s (default: the configured one)"),
				}, "input"),
			},
			call: toolResolveInput,
		},
	}
	for _, t := range read {
		s.register(t)
	}
	registerSessionTools(s)
	registerStateTools(s)
}

// argError marks a problem with the arguments of a tool call.
func argError(msg string) error { return &toolArgError{msg} }

type toolArgError struct{ msg string }

func (e *toolArgError) Error() string { return e.msg }

func toolRead(s *mcpServer, raw json.RawMessage) (any, error) {
	var a struct {
		Target     string `json:"target"`
		StripLinks bool   `json:"strip_links"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, argError("bad arguments: " + err.Error())
	}
	if strings.TrimSpace(a.Target) == "" {
		return nil, argError("target is required")
	}
	cfg, _ := loadConfig()
	src, search := normalizeInput(a.Target, cfg.SearchEngine)
	if src == "" {
		return nil, argError("empty target")
	}
	md, err := load(src)
	if err != nil {
		return nil, err
	}
	if a.StripLinks {
		md = stripLinks(md)
	}
	title := markdownTitle(md, src)
	return map[string]any{
		"target":   a.Target,
		"url":      src,
		"search":   search,
		"title":    title,
		"bytes":    len(md),
		"markdown": md,
	}, nil
}

func toolRender(s *mcpServer, raw json.RawMessage) (any, error) {
	var a struct {
		Target string `json:"target"`
		Width  int    `json:"width"`
		ANSI   bool   `json:"ansi"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, argError("bad arguments: " + err.Error())
	}
	if strings.TrimSpace(a.Target) == "" {
		return nil, argError("target is required")
	}
	cfg, _ := loadConfig()
	src, _ := normalizeInput(a.Target, cfg.SearchEngine)
	if src == "" {
		return nil, argError("empty target")
	}
	md, err := load(src)
	if err != nil {
		return nil, err
	}
	width := a.Width
	if width <= 0 {
		width = cfg.Width
	}
	if width <= 0 {
		width = 100
	}
	doc := renderDoc(md, width, cfg)
	text := strings.Join(doc.Plain, "\n")
	if a.ANSI {
		text = strings.Join(doc.Lines, "\n")
	}
	return map[string]any{
		"target":   a.Target,
		"url":      src,
		"width":    width,
		"lines":    len(doc.Lines),
		"headings": len(doc.Heads),
		"links":    len(uniqueLinks(doc)),
		"text":     text,
	}, nil
}

func toolLinks(s *mcpServer, raw json.RawMessage) (any, error) {
	var a struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, argError("bad arguments: " + err.Error())
	}
	if strings.TrimSpace(a.Target) == "" {
		return nil, argError("target is required")
	}
	cfg, _ := loadConfig()
	src, _ := normalizeInput(a.Target, cfg.SearchEngine)
	if src == "" {
		return nil, argError("empty target")
	}
	md, err := load(src)
	if err != nil {
		return nil, err
	}
	doc := renderDoc(md, cfg.Width, cfg)
	links := make([]map[string]any, 0, len(doc.Links))
	for _, l := range uniqueLinks(doc) { // one entry per destination, reading order
		if l.ID < 0 || l.ID >= len(doc.URLs) {
			continue
		}
		text := ""
		if l.Line >= 0 && l.Line < len(doc.Plain) {
			runes := []rune(doc.Plain[l.Line])
			lo, hi := clamp(l.From, 0, len(runes)), clamp(l.To, 0, len(runes))
			text = string(runes[lo:max(lo, hi)])
		}
		href := doc.URLs[l.ID]
		abs, ok := resolveLink(src, href)
		entry := map[string]any{"text": text, "href": href, "url": abs}
		switch {
		case !ok:
			entry["kind"] = "unsupported"
		case strings.HasPrefix(href, "#"):
			entry["kind"] = "anchor"
		case strings.HasPrefix(abs, "file://"):
			entry["kind"] = "file"
		default:
			entry["kind"] = "web"
		}
		links = append(links, entry)
	}
	return map[string]any{
		"target": a.Target,
		"url":    src,
		"count":  len(links),
		"links":  links,
	}, nil
}

func toolResolveInput(s *mcpServer, raw json.RawMessage) (any, error) {
	var a struct {
		Input  string `json:"input"`
		Engine string `json:"engine"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, argError("bad arguments: " + err.Error())
	}
	if strings.TrimSpace(a.Input) == "" {
		return nil, argError("input is required")
	}
	engine := a.Engine
	if engine == "" {
		cfg, _ := loadConfig()
		engine = cfg.SearchEngine
	}
	dst, search := normalizeInput(a.Input, engine)
	return map[string]any{
		"input":           a.Input,
		"url":             dst,
		"search":          search,
		"engine":          engine,
		"engine_template": engineTemplate(engine),
	}, nil
}

// markdownTitle reads the page title wr put at the top of the Markdown.
func markdownTitle(md, src string) string {
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
		break // the first content line is not a heading: no page title
	}
	if u, err := url.Parse(src); err == nil {
		return u.Host
	}
	return src
}

// uniqueLinks counts each destination once, in reading order, like the reader's
// link focus does.
func uniqueLinks(doc *Doc) []LinkSpan {
	seen := map[int]bool{}
	var out []LinkSpan
	for _, l := range doc.Links {
		if !seen[l.ID] {
			seen[l.ID] = true
			out = append(out, l)
		}
	}
	return out
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
