package version

var (
	// Version is the semantic version tag (e.g. "0.1.0").
	Version = "dev"

	// Commit is the short git commit hash.
	Commit = "unknown"

	// Date is the UTC build timestamp.
	Date = "unknown"

	GoARM = ""
)

// Full returns a human-readable version string.
func Full() string {
	return Version + " (" + Commit + ") " + Date
}

// UserAgent returns a string suitable for HTTP User-Agent headers.
func UserAgent(component string) string {
	return "Spectra-" + component + "/" + Version
}
