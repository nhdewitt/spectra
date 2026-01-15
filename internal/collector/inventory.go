package collector

type AppInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Vendor  string `json:"vendor,omitempty"`
}
