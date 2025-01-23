package witcli

import (
	"runtime/debug"
	"sync"
)

// Version returns the version string of this module.
func Version() string {
	return versionString()
}

var versionString = sync.OnceValue(func() string {
	build, ok := debug.ReadBuildInfo()
	if !ok {
		return "(none)"
	}
	version := build.Main.Version
	var revision string
	for _, s := range build.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		}
	}
	if version == "" {
		version = "(none)"
	}
	versionString := version
	if revision != "" {
		versionString += " (" + revision + ")"
	}
	return versionString
})
