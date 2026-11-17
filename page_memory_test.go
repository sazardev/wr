package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestPageMemoryRoundTripAndBounds(t *testing.T) {
	m := &model{cfg: defaultConfig()}
	m.rememberPage("https://a.test/1", "one", time.Now())
	if p, ok := m.recallPage("https://a.test/1"); !ok || p.md != "one" {
		t.Fatalf("recall: %+v %v", p, ok)
	}
	m.forgetPage("https://a.test/1")
	if _, ok := m.recallPage("https://a.test/1"); ok {
		t.Error("a forgotten page is still in memory")
	}

	for i := 0; i < pageMemoryPages+5; i++ {
		m.rememberPage(fmt.Sprintf("https://a.test/%d", i), "body", time.Now())
	}
	if len(m.pages) > pageMemoryPages {
		t.Errorf("the memory cache grew to %d pages", len(m.pages))
	}
	if m.memBytes > pageMemoryBytes {
		t.Errorf("the memory cache grew to %d bytes", m.memBytes)
	}
	m.forgetAllPages()
	if m.pages != nil || m.memBytes != 0 {
		t.Error("forgetAllPages must empty the cache")
	}
}

func TestPageMemoryRespectsTheCacheSetting(t *testing.T) {
	m := &model{cfg: defaultConfig()}
	m.cfg.Cache = false
	m.rememberPage("https://a.test/x", "body", time.Now())
	if _, ok := m.recallPage("https://a.test/x"); ok {
		t.Error("with the cache off, pages must not stay in memory")
	}
}

func TestBackIsServedFromMemory(t *testing.T) {
	isolateAll(t)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><head><title>Page</title></head><body><article><h1>Page</h1><p>`)
		for i := 0; i < 40; i++ {
			fmt.Fprint(w, "Filler words that make this page worth reading. ")
		}
		fmt.Fprintf(w, `</p><a href="/b">next</a></article></body></html>`)
	}))
	defer srv.Close()

	cfg := defaultConfig()
	cfg.Animations = false
	m := newModel(srv.URL, cfg, false, nil)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	settle(m, m.Init())
	if hits.Load() != 1 || m.src != srv.URL {
		t.Fatalf("first load: hits=%d src=%q", hits.Load(), m.src)
	}
	// Remove the disk copy: what comes back must come from memory.
	if err := cacheDelete(srv.URL); err != nil {
		t.Fatal(err)
	}
	settle(m, m.startNav(navPush, srv.URL+"/b", 0, false))
	if hits.Load() != 2 {
		t.Fatalf("second page: hits=%d", hits.Load())
	}
	settle(m, m.goBack())
	if m.src != srv.URL {
		t.Fatalf("back went to %q", m.src)
	}
	if hits.Load() != 2 {
		t.Errorf("back hit the network (%d requests): it must use the session cache", hits.Load())
	}
	if m.doc == nil || m.doc.Heads[0].Text != "Page" {
		t.Error("the remembered page did not render")
	}
}
