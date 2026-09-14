// Package otelxredis provides OpenTelemetry metrics instrumentation for xredis.
package otelxredis

import "runtime/debug"

// ScopeName is the OpenTelemetry instrumentation scope name used by this module.
const ScopeName = "github.com/mkbeh/xredis/extra/otelxredis"

// Version returns the module version of otelxredis.
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
