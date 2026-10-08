package main

import (
	"runtime/debug"
	"strings"
)

// version is set at build time (-ldflags "-X main.version=v0.1.0"). When it is
// not, the module version recorded by `go install ...@vX.Y.Z` is used.
var version = ""

func versionString() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}

func versionLine() string {
	v := versionString()
	rev := ""
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 7 {
				rev = s.Value[:7]
			}
		}
	}
	if rev != "" && !strings.Contains(v, rev) && v == "dev" {
		return "wr dev (" + rev + ")"
	}
	return "wr " + v
}
