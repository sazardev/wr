package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func updModel(t *testing.T, version string, w, h int, tweak func(*Config)) *model {
	t.Helper()
	withVersion(t, version)
	isolateAll(t)
	old := updateEnabled
	updateEnabled = true
	t.Cleanup(func() { updateEnabled = old })
	cfg := defaultConfig()
	cfg.Animations = false
	if tweak != nil {
		tweak(&cfg)
	}
	return loaded(newModel("https://t.test/x", cfg, false, nil), w, h)
}

var newRelease = &releaseInfo{Tag: "v0.6.0", Body: releasePleaseBody, URL: "https://github.com/sazardev/wr/releases/tag/v0.6.0"}

func found(m *model, announce bool) {
	m.Update(updateCheckedMsg{res: checkResult{Latest: newRelease, Newer: true, Announce: announce}})
}

func screen(m *model) string { return ansi.Strip(m.render()) }

func TestFooterDoesNotShowTheVersion(t *testing.T) {
	m := updModel(t, "v0.5.1", 120, 30, nil)
	if strings.Contains(lastRow(m), "v0.5.1") {
		t.Errorf("the footer stays free of the version:\n%s", lastRow(m))
	}
	// the version lives in the menu title and on the start page
	press(m, "m")
	if !strings.Contains(screen(m), "wr v0.5.1") {
		t.Errorf("the menu title carries the version:\n%s", screen(m))
	}
}

func TestMenuShowsTheVersionAndTheUpdateEntry(t *testing.T) {
	m := updModel(t, "v0.5.1", 100, 40, nil)
	press(m, "m")
	s := screen(m)
	if !strings.Contains(s, "wr v0.5.1") || !strings.Contains(s, "Update wr / what's new") {
		t.Errorf("menu:\n%s", s)
	}
	press(m, "esc")
	found(m, false)
	press(m, "m")
	if !strings.Contains(screen(m), "v0.6.0 available") {
		t.Errorf("the menu entry must say what is available:\n%s", screen(m))
	}
}

func TestANewVersionIsAnnouncedWithoutInterrupting(t *testing.T) {
	m := updModel(t, "v0.5.1", 120, 30, nil) // whats_new is off by default
	found(m, true)
	if m.mode != modeRead {
		t.Fatalf("with whats_new off nothing may open on its own, mode = %v", m.mode)
	}
	if !strings.Contains(m.toast, "v0.6.0 is available") || !strings.Contains(m.toast, "U") {
		t.Errorf("toast = %q", m.toast)
	}
	if !strings.Contains(lastRow(m), "↑ v0.6.0 (U)") {
		t.Errorf("the footer must keep the reminder:\n%s", lastRow(m))
	}
	// announced already: no toast on the next sighting, the chip stays
	m = updModel(t, "v0.5.1", 120, 30, nil)
	found(m, false)
	if m.toast != "" {
		t.Errorf("an announced version must not toast again: %q", m.toast)
	}
}

func TestWhatsNewWindowOpensOnlyWhenAsked(t *testing.T) {
	on := func(c *Config) { c.WhatsNew = true }
	m := updModel(t, "v0.5.1", 110, 40, on)
	found(m, true)
	if m.mode != modeUpdate {
		t.Fatalf("whats_new on: the window must open, mode = %v", m.mode)
	}
	s := screen(m)
	for _, want := range []string{"wr v0.6.0 is available", "You have v0.5.1", "Features", "agent harness", "update now", "later"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, "670841b") || strings.Contains(s, "](") {
		t.Errorf("commit links must not show in the notes:\n%s", s)
	}
	press(m, "esc")
	if m.mode != modeRead {
		t.Error("esc must close the window")
	}

	// never over something you are typing
	m = updModel(t, "v0.5.1", 110, 40, on)
	press(m, "/")
	found(m, true)
	if m.mode != modeSearch || !strings.Contains(m.toast, "available") {
		t.Errorf("mid-search the notice must be a toast: mode=%v toast=%q", m.mode, m.toast)
	}
}

func TestUKeyOpensTheWindowAnyTime(t *testing.T) {
	m := updModel(t, "v0.5.1", 110, 40, nil)
	found(m, false)
	press(m, "U")
	if m.mode != modeUpdate || !strings.Contains(screen(m), "agent harness") {
		t.Fatalf("U must show the notes of the newer release:\n%s", screen(m))
	}
	press(m, "U")
	if m.mode != modeRead {
		t.Error("U again closes it")
	}
	// up to date: says so
	m = updModel(t, "v0.6.0", 110, 40, nil)
	m.Update(updateCheckedMsg{res: checkResult{Latest: newRelease}})
	press(m, "U")
	if s := screen(m); !strings.Contains(s, "You are up to date.") || !strings.Contains(s, "check now") {
		t.Errorf("up to date:\n%s", s)
	}
}

func TestEnterStartsTheUpdateAndTheResultIsShown(t *testing.T) {
	m := updModel(t, "v0.5.1", 110, 40, nil)
	found(m, false)
	press(m, "U")
	_, cmd := m.keyUpdate("enter")
	if cmd == nil || m.upd.phase != updBusy {
		t.Fatalf("enter must start the update: phase=%v cmd=%v", m.upd.phase, cmd)
	}
	if s := screen(m); !strings.Contains(s, "Downloading and installing v0.6.0") {
		t.Errorf("busy window:\n%s", s)
	}
	if _, again := m.keyUpdate("enter"); again != nil {
		t.Error("a second enter must not start a second download")
	}

	press(m, "U") // the window can be closed while it works, and reopened
	m.Update(updateDoneMsg{tag: "v0.6.0"})
	if !strings.Contains(m.toast, "installed") || !strings.Contains(lastRow(m), "restart") {
		t.Errorf("success: toast=%q footer=%q", m.toast, lastRow(m))
	}
	if s := screen(m); !strings.Contains(s, "Restart wr to start using it.") {
		t.Errorf("done window:\n%s", s)
	}
}

func TestAFailedUpdateExplainsItself(t *testing.T) {
	m := updModel(t, "v0.5.1", 110, 40, nil)
	found(m, false)
	press(m, "U")
	m.keyUpdate("enter")
	m.Update(updateDoneMsg{tag: "v0.6.0", err: errTest})
	if m.upd.phase != updFailed || !m.toastWarn {
		t.Errorf("phase=%v warn=%v", m.upd.phase, m.toastWarn)
	}
	if s := screen(m); !strings.Contains(s, "render failed") || !strings.Contains(s, "update now") {
		t.Errorf("a failure must show why and allow trying again:\n%s", s)
	}
}

func TestFirstRunAfterUpdateShowsWhatsNewOnlyIfEnabled(t *testing.T) {
	just := checkResult{Latest: newRelease, JustUpdate: newRelease}
	m := updModel(t, "v0.6.0", 110, 40, func(c *Config) { c.WhatsNew = true })
	m.Update(updateCheckedMsg{res: just})
	if s := screen(m); m.mode != modeUpdate || !strings.Contains(s, "What's new in wr v0.6.0") || !strings.Contains(s, "You are now on v0.6.0") {
		t.Errorf("mode=%v\n%s", m.mode, s)
	}
	m = updModel(t, "v0.6.0", 110, 40, nil)
	m.Update(updateCheckedMsg{res: just})
	if m.mode != modeRead || m.toast != "" {
		t.Errorf("whats_new off: nothing may appear (mode=%v toast=%q)", m.mode, m.toast)
	}
}

func TestBackgroundCheckHonorsTheSwitches(t *testing.T) {
	m := updModel(t, "v0.5.1", 100, 30, nil)
	if m.startUpdateCheck() == nil {
		t.Error("the default is to check")
	}
	m = updModel(t, "v0.5.1", 100, 30, func(c *Config) { c.UpdateCheck = false })
	if m.startUpdateCheck() != nil {
		t.Error("update_check = false must not check")
	}
	m = updModel(t, "v0.5.1", 100, 30, nil)
	updateEnabled = false
	if m.startUpdateCheck() != nil {
		t.Error("outside the interactive reader nothing checks")
	}
	// and a manual miss while offline is explained
	updateEnabled = true
	m.Update(updateCheckedMsg{res: checkResult{Err: errTest}, manual: true})
	if m.upd.phase != updFailed || !strings.Contains(m.upd.msg, "could not check") {
		t.Errorf("phase=%v msg=%q", m.upd.phase, m.upd.msg)
	}
}

func TestDefaultsAreOnForTheCheckAndOffForTheWindow(t *testing.T) {
	c := defaultConfig()
	if !c.UpdateCheck || c.WhatsNew {
		t.Errorf("update_check should default on and whats_new off: %+v %+v", c.UpdateCheck, c.WhatsNew)
	}
}
