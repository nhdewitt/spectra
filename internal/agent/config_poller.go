package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nhdewitt/spectra/internal/logging"
)

func (a *Agent) runConfigPoller(ctx context.Context) {
	// Fetch once right away so the ignore lists are normally in
	// place before the collectors start at the next minute boundary.
	a.fetchAndApplyConfig(ctx)

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.fetchAndApplyConfig(ctx)
		}
	}
}

func (a *Agent) fetchAndApplyConfig(ctx context.Context) {
	url := fmt.Sprintf("%s/api/v1/agent/config", a.Config.BaseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		a.Logger.Debug("config poll request failed", "error", err)
		return
	}
	a.setHeaders(req)
	req.Header.Del("Content-Encoding")

	resp, err := a.Client.Do(req)
	if err != nil {
		a.Logger.Debug("config poll failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var config map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		a.Logger.Warn("failed to decode remote config", "error", err)
		return
	}

	if raw, ok := config["log_level"]; ok {
		var level string
		if json.Unmarshal(raw, &level) == nil && level != "" {
			parsed := logging.ParseLevel(level)
			if parsed != a.Logger.ConsoleLevel.Level() {
				previous := a.Logger.ConsoleLevel.Level()
				a.Logger.SetConsoleLevel(parsed)
				a.Logger.SetFileLevel(parsed)
				a.Logger.Info("log level updated from remote config",
					"previous", previous, "level", parsed)
			}
		}
	}
	a.applyIgnoreLists(config)
}

// applyIgnoreLists replaces the agent's ignore lists from the ignored_filesystems
// and ignored_interfacces keys. A missing key means nothing is ignored for that
// kind, since the UI deletes the key when the list is emptied.
func (a *Agent) applyIgnoreLists(config map[string]json.RawMessage) {
	filesystems := a.stringList(config, "ignored_filesystems")
	interfaces := a.stringList(config, "ignored_interfaces")

	next := newIgnoreLists(filesystems, interfaces)
	if current := a.ignore.Load(); current != nil && current.equal(next) {
		return
	}
	if a.ignore.Swap(next) == nil && len(filesystems) == 0 && len(interfaces) == 0 {
		return
	}
	a.Logger.Info("ignore lists updated from remote config",
		"filesystems", filesystems, "interfaces", interfaces)
}

// stringList decodes config[key] as a list of strings. A missing key or a value of
// the wrong shape yields nil.
func (a *Agent) stringList(config map[string]json.RawMessage, key string) []string {
	raw, ok := config[key]
	if !ok {
		return nil
	}

	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		a.Logger.Warn("ignoring malformed remote config value", "key", key, "error", err)
		return nil
	}

	return list
}
