package main

import (
	"fmt"
	"io"
	"runtime"
	"runtime/debug"
)

// version is stamped at link time via -ldflags "-X main.version=<value>".
// Falls back to "dev" for go run / unstamped builds.
var version = "dev"

// BuildInfo is the resolved build-time identity of the binary.
type BuildInfo struct {
	Version   string // human release tag (e.g. "1.2.3"), or "dev"
	Revision  string // short git SHA, from runtime/debug VCS info
	BuildDate string // RFC 3339 commit time, from runtime/debug VCS info
	GoVersion string
}

// Info returns the resolved build-time identity of this binary, combining the
// ldflags-stamped version with VCS data auto-embedded by the Go toolchain.
func Info() BuildInfo {
	bi := BuildInfo{Version: version, GoVersion: runtime.Version()}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return bi
	}
	// Prefer the module version when ldflags wasn't used (e.g. `go install module@vX.Y.Z`).
	if bi.Version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		bi.Version = info.Main.Version
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 7 {
				bi.Revision = s.Value[:7]
			} else {
				bi.Revision = s.Value
			}
		case "vcs.time":
			bi.BuildDate = s.Value
		}
	}
	return bi
}

// Print writes a human-readable version block to w.
func Print(w io.Writer) {
	bi := Info()
	_, _ = fmt.Fprintf(w, `do-obsd (DigitalOcean Observability Supervisor)
Version:    %s
Revision:   %s
Build Date: %s
Go Version: %s
`, bi.Version, bi.Revision, bi.BuildDate, bi.GoVersion)
}
