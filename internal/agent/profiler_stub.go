//go:build !spectraprof

package agent

import "context"

// startProfiler is a no-op in release builds. The profiling code is
// excluded at compile time rather than gated at runtime, so a
// self-update can never hand a production host a binary that can be
// made to profile.
func (a *Agent) startProfiler(context.Context) {}
