package main

import (
	"container/list"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// In-memory cache of rendered documents. Rendering a page is the most
// expensive thing wr does after extraction: a resize re-renders everything, and
// so does going back to a page you just read. The key is the Markdown itself
// (it is already in memory), the width, the layout options and the palette
// generation, so a document is only reused when it would come out identical.
//
// The cache is bounded by entries and by the size of the documents it keeps:
// a handful of very long pages cannot grow it without limit.

type renderKey struct {
	md    string
	width int
	sig   string
	pal   uint64
}

type renderEntry struct {
	key   renderKey
	doc   *Doc
	bytes int
}

type renderCache struct {
	mu      sync.Mutex
	order   *list.List
	entries map[renderKey]*list.Element
	bytes   int
	maxDocs int
	maxSize int
}

func newRenderCache(maxDocs, maxSize int) *renderCache {
	return &renderCache{
		order:   list.New(),
		entries: make(map[renderKey]*list.Element),
		maxDocs: maxDocs,
		maxSize: maxSize,
	}
}

func (c *renderCache) get(k renderKey) *Doc {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.entries[k]; ok {
		c.order.MoveToFront(el)
		return el.Value.(*renderEntry).doc
	}
	return nil
}

func (c *renderCache) put(k renderKey, doc *Doc) {
	size := docSize(doc)
	if size > c.maxSize {
		return // one enormous page: caching it would evict everything else
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.entries[k]; ok {
		c.order.MoveToFront(el)
		return
	}
	c.entries[k] = c.order.PushFront(&renderEntry{key: k, doc: doc, bytes: size})
	c.bytes += size
	for c.order.Len() > c.maxDocs || c.bytes > c.maxSize {
		back := c.order.Back()
		if back == nil {
			break
		}
		ent := back.Value.(*renderEntry)
		c.order.Remove(back)
		delete(c.entries, ent.key)
		c.bytes -= ent.bytes
	}
}

// docSize approximates the memory a document holds, so the byte budget tracks
// the real cost rather than the number of entries.
func docSize(d *Doc) int {
	n := 0
	for _, l := range d.Lines {
		n += len(l)
	}
	for _, l := range d.Plain {
		n += len(l)
	}
	for _, h := range d.Heads {
		n += len(h.Text) + 16
	}
	for _, u := range d.URLs {
		n += len(u)
	}
	return n + len(d.Meta)*8 + len(d.Links)*16
}

// renderDocCached is renderDoc with the in-memory cache in front. The returned
// Document is shared and must be treated as read-only (no caller mutates it).
func renderDocCached(md string, width int, cfg Config) *Doc {
	key := renderKey{md: md, width: width, sig: renderSignature(cfg), pal: paletteGen.Load()}
	if d := renderedDocs.get(key); d != nil {
		return d
	}
	d := renderDoc(md, width, cfg)
	renderedDocs.put(key, d)
	return d
}

const (
	renderCacheDocs = 12
	renderCacheSize = 48 << 20 // 48 MB of rendered lines
)

var renderedDocs = newRenderCache(renderCacheDocs, renderCacheSize)

// paletteGen moves whenever the active colors change, so a document rendered
// with the old palette is never served after switching the accent or the
// terminal background.
var paletteGen atomic.Uint64

func bumpPalette() { paletteGen.Add(1) }

// renderSignature covers every option renderDoc reads (width is part of the
// key). Anything that changes the output of a document belongs here.
func renderSignature(cfg Config) string {
	var b strings.Builder
	b.Grow(32)
	b.WriteString(cfg.Links)
	b.WriteByte('|')
	b.WriteString(strconv.Itoa(cfg.Spacing))
	b.WriteByte('|')
	if cfg.HeadingRules {
		b.WriteByte('h')
	}
	if cfg.CodeFrame {
		b.WriteByte('c')
	}
	if cfg.GuessCodeLang {
		b.WriteByte('g')
	}
	if cfg.Braille {
		b.WriteByte('b')
	}
	return b.String()
}
