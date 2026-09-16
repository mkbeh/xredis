package otelxredis

import "runtime/debug"

// ScopeName is the OpenTelemetry instrumentation scope name used by this package.
const ScopeName = "github.com/mkbeh/xredis/extra/otelxredis"

// Version returns the version of the otelxredis module from Go build
// information.
//
// It returns "unknown" when build information is unavailable or does not
// contain this module.
func Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	if info.Main.Path == ScopeName {
		return info.Main.Version
	}

	for _, dep := range info.Deps {
		if dep.Path == ScopeName {
			return dep.Version
		}
	}

	return "unknown"
}
