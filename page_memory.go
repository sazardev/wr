package main

import "time"

// A small in-memory cache of the pages of this session. The disk cache already
// makes a page you read open in a couple of milliseconds; this layer removes
// even that read, so going back and forward is instant and does not touch the
// filesystem. It is bounded by pages and by bytes, and only used when the disk
// cache is on: turning the cache off keeps every page out of memory too.

const (
	pageMemoryPages = 24
	pageMemoryBytes = 24 << 20
	pageMemoryFresh = 2 * time.Minute // younger than this: no background refresh
)

type memoPage struct {
	md    string
	saved time.Time
	seq   uint64
}

func (m *model) recallPage(src string) (memoPage, bool) {
	if !m.cfg.Cache || m.pages == nil {
		return memoPage{}, false
	}
	p, ok := m.pages[src]
	if !ok {
		return memoPage{}, false
	}
	m.memSeq++
	p.seq = m.memSeq
	m.pages[src] = p
	return p, true
}

func (m *model) rememberPage(src, md string, saved time.Time) {
	if !m.cfg.Cache || !isHTTP(src) || md == "" || len(md) > pageMemoryBytes {
		return
	}
	if m.pages == nil {
		m.pages = make(map[string]memoPage, pageMemoryPages)
	}
	if old, ok := m.pages[src]; ok {
		m.memBytes -= len(old.md)
	}
	m.memSeq++
	m.pages[src] = memoPage{md: md, saved: saved, seq: m.memSeq}
	m.memBytes += len(md)
	for len(m.pages) > pageMemoryPages || m.memBytes > pageMemoryBytes {
		oldest := ""
		var minSeq uint64
		for k, p := range m.pages {
			if oldest == "" || p.seq < minSeq {
				oldest, minSeq = k, p.seq
			}
		}
		if oldest == "" {
			break
		}
		m.memBytes -= len(m.pages[oldest].md)
		delete(m.pages, oldest)
	}
}

func (m *model) forgetPage(src string) {
	if p, ok := m.pages[src]; ok {
		m.memBytes -= len(p.md)
		delete(m.pages, src)
	}
}

func (m *model) forgetAllPages() {
	m.pages = nil
	m.memBytes = 0
}

// cacheInfo is cacheStats for the UI: the menu is redrawn on every keypress (and
// its filter asks for the items once per entry), so scanning the cache
// directory each time would put disk I/O in the middle of a keypress. A short
// time-to-live keeps the numbers current without the cost; anything that
// changes the cache invalidates it explicitly.
func (m *model) cacheInfo() (int, int64) {
	if m.cacheStatAt.IsZero() || m.now.Sub(m.cacheStatAt) >= 2*time.Second || m.now.Before(m.cacheStatAt) {
		m.cacheStatPages, m.cacheStatBytes = cacheStats()
		m.cacheStatAt = m.now
	}
	return m.cacheStatPages, m.cacheStatBytes
}

func (m *model) forgetCacheInfo() { m.cacheStatAt = time.Time{} }
