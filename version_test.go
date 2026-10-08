package main

import (
	"strings"
	"testing"
)

func TestVersionString(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = "v1.2.3"
	if versionString() != "v1.2.3" || versionLine() != "wr v1.2.3" {
		t.Errorf("%q %q", versionString(), versionLine())
	}
	version = ""
	if v := versionString(); v == "" {
		t.Error("the version is never empty")
	}
	if !strings.HasPrefix(versionLine(), "wr ") {
		t.Errorf("%q", versionLine())
	}
}
