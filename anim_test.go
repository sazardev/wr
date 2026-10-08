package main

import (
	"math"
	"testing"
	"time"
)

func TestSpringConvergesWithoutOvershoot(t *testing.T) {
	s := spring{target: 100}
	maxPos := 0.0
	for i := 0; i < 600 && !s.settled(); i++ {
		s.step(1)
		maxPos = math.Max(maxPos, s.pos)
	}
	if !s.settled() {
		t.Fatalf("did not settle: pos=%v vel=%v", s.pos, s.vel)
	}
	if maxPos > 100.5 {
		t.Errorf("critically damped spring overshot: max %.2f", maxPos)
	}
}

func TestSpringSettlesQuickly(t *testing.T) {
	s := spring{target: 40}
	frames := 0
	for !s.settled() && frames < 600 {
		s.step(1)
		frames++
	}
	// 60 fps: the scroll must feel snappy, well under half a second
	if d := time.Duration(frames) * frameFast; d > 500*time.Millisecond {
		t.Errorf("took %v to settle", d)
	}
}

func TestSpringStepCatchesUp(t *testing.T) {
	a, b := spring{target: 50}, spring{target: 50}
	a.step(3)
	b.step(1)
	b.step(1)
	b.step(1)
	if a.pos != b.pos {
		t.Errorf("step(3) != 3x step(1): %v vs %v", a.pos, b.pos)
	}
}

func TestSpringSnap(t *testing.T) {
	s := spring{pos: 3, vel: 9, target: 70}
	s.snap()
	if s.pos != 70 || s.vel != 0 || !s.settled() {
		t.Errorf("%+v", s)
	}
}

func TestProgressSince(t *testing.T) {
	now := time.Now()
	if progressSince(time.Time{}, now, time.Second) != 1 {
		t.Error("a zero start means already finished")
	}
	if got := progressSince(now, now, time.Second); got != 0 {
		t.Errorf("start: %v", got)
	}
	if got := progressSince(now.Add(-500*time.Millisecond), now, time.Second); math.Abs(got-0.5) > 0.001 {
		t.Errorf("half: %v", got)
	}
	if got := progressSince(now.Add(-time.Hour), now, time.Second); got != 1 {
		t.Errorf("clamped: %v", got)
	}
}

func TestEaseOutCubic(t *testing.T) {
	if easeOutCubic(0) != 0 || easeOutCubic(1) != 1 {
		t.Fatal("endpoints")
	}
	if easeOutCubic(0.5) <= 0.5 {
		t.Error("ease-out starts fast: f(0.5) should be above 0.5")
	}
	if easeOutCubic(-3) != 0 || easeOutCubic(9) != 1 {
		t.Error("input is clamped")
	}
}

func TestTyped(t *testing.T) {
	start := time.Now()
	if s, done := typed("αβγδ", start, start.Add(2*typeSpeed), typeSpeed); s != "αβ" || done {
		t.Errorf("rune-aware prefix: %q done=%v", s, done)
	}
	if s, done := typed("hi", start, start.Add(time.Hour), typeSpeed); s != "hi" || !done {
		t.Errorf("finished: %q %v", s, done)
	}
	if s, done := typed("hi", time.Time{}, start, typeSpeed); s != "hi" || !done {
		t.Errorf("zero start: %q %v", s, done)
	}
}
