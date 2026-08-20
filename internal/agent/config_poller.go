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
					"previous", previous, "level", level)
			}
		}
	}
}
