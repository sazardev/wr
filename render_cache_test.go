package main

import "testing"

func TestRenderCacheReusesIdenticalDocuments(t *testing.T) {
	old := renderedDocs
	renderedDocs = newRenderCache(4, 1<<20)
	t.Cleanup(func() {
		renderedDocs = old
		setAccent("blue")
	})

	cfg := defaultConfig()
	md := "# A\n\ntext with `code` and a [link](https://ex.test)\n"

	a := renderDocCached(md, 80, cfg)
	if b := renderDocCached(md, 80, cfg); a != b {
		t.Fatal("the same input must reuse the same rendered document")
	}
	if c := renderDocCached(md, 81, cfg); c == a {
		t.Error("a different width must render again")
	}
	cfg.HeadingRules = false
	if d := renderDocCached(md, 80, cfg); d == a {
		t.Error("a layout change must render again")
	}
	setAccent("red")
	if e := renderDocCached(md, 80, cfg); e == a {
		t.Error("a palette change must render again")
	}
}

func TestRenderCacheEvictsLeastRecentlyUsed(t *testing.T) {
	c := newRenderCache(2, 1<<20)
	cfg := defaultConfig()
	doc := func(s string) *Doc { return &Doc{Lines: []string{s}} }

	c.put(renderKey{md: "a", width: 80, sig: renderSignature(cfg)}, doc("a"))
	c.put(renderKey{md: "b", width: 80, sig: renderSignature(cfg)}, doc("b"))
	// touch a, so b is the oldest
	if c.get(renderKey{md: "a", width: 80, sig: renderSignature(cfg)}) == nil {
		t.Fatal("a was not cached")
	}
	c.put(renderKey{md: "c", width: 80, sig: renderSignature(cfg)}, doc("c"))
	if c.get(renderKey{md: "b", width: 80, sig: renderSignature(cfg)}) != nil {
		t.Error("the least recently used entry must be evicted")
	}
	if c.get(renderKey{md: "a", width: 80, sig: renderSignature(cfg)}) == nil {
		t.Error("the recently used entry was evicted")
	}
}

func TestRenderCacheKeepsTheSharingReadOnly(t *testing.T) {
	old := renderedDocs
	renderedDocs = newRenderCache(2, 1<<20)
	t.Cleanup(func() { renderedDocs = old })

	md := "# Title\n\nbody\n"
	a := renderDocCached(md, 60, defaultConfig())
	b := renderDocCached(md, 60, defaultConfig())
	if len(a.Lines) != len(b.Lines) || a.Heads[0].Text != b.Heads[0].Text {
		t.Fatal("the cached document changed between calls")
	}
}
