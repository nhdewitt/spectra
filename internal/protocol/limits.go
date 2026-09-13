package protocol

// Request body ceilings, shared so the two sides cannot drift apart.
//
// Both ends depend on them: the server rejects anything larger, and
// the agent needs to know what it is allowed to produce.
const (
	// MaxCommandResultBytes bounds a command result after decompression.
	// Diagnostic output is the large case. FetchLogs trims its result
	// to fit under this before sending.
	MaxCommandResultBytes = 4 << 20

	// MaxMetricsBytes bounds a metric batch after decompression. It
	// must stay above what an agent can produce in one request, since
	// an agent requeues any rejected batch. A full maxUploadChunk is
	// roughly 2MB decompressed, leaving 8x headroom.
	MaxMetricsBytes = 16 << 20
)
