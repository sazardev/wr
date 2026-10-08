package main

import (
	"math"
	"time"

	"github.com/charmbracelet/harmonica"
)

// Animation helpers. Everything moves with spring physics (Charm's harmonica)
// and only while something is actually changing: when the screen settles, the
// tick stops and the program idles at 0% CPU.

const (
	frameFast = time.Second / 60      // scrolling, reveal, panels, loading wave
	frameSlow = 80 * time.Millisecond // background spinner only

	revealDur = 140 * time.Millisecond
	panelDur  = 120 * time.Millisecond
	typeSpeed = 4 * time.Millisecond // per character, for toasts

	// A notice should confirm and get out of the way (warnings linger a bit).
	toastLinger     = 1200 * time.Millisecond
	toastWarnLinger = 2500 * time.Millisecond
)

// Critically damped (no bounce: bouncing text is unreadable). A one-line step
// reaches its row in ~60 ms and a 300-line jump in ~330 ms; the invisible last
// fraction of a line takes a little longer before the ticks stop.
var scrollSpring = harmonica.NewSpring(harmonica.FPS(60), 28, 1)

type spring struct{ pos, vel, target float64 }

// step advances the spring n fixed 1/60 s frames (n > 1 catches up after a
// slow frame so the motion keeps its real-time speed).
func (s *spring) step(n int) {
	for i := 0; i < n; i++ {
		s.pos, s.vel = scrollSpring.Update(s.pos, s.vel, s.target)
	}
}

// settled: close enough that the difference is invisible (5% of a line, which
// is well under a quarter of a scrollbar cell) and nearly stopped.
func (s *spring) settled() bool {
	return math.Abs(s.pos-s.target) < 0.05 && math.Abs(s.vel) < 0.5
}

func (s *spring) snap() { s.pos, s.vel = s.target, 0 }

func clamp01(t float64) float64 { return math.Min(math.Max(t, 0), 1) }

func easeOutCubic(t float64) float64 { return 1 - math.Pow(1-clamp01(t), 3) }

// progressSince is how far (0..1) an animation of length d that started at
// `start` has got at `now`. A zero start means "no animation": already done.
func progressSince(start, now time.Time, d time.Duration) float64 {
	if start.IsZero() {
		return 1
	}
	return clamp01(float64(now.Sub(start)) / float64(d))
}

// typed returns the part of s that has been "typed" so far (rune aware).
func typed(s string, start, now time.Time, perChar time.Duration) (shown string, done bool) {
	r := []rune(s)
	if start.IsZero() || perChar <= 0 {
		return s, true
	}
	n := int(now.Sub(start) / perChar)
	if n >= len(r) {
		return s, true
	}
	return string(r[:max(n, 0)]), false
}
