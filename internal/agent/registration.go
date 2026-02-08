package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nhdewitt/spectra/internal/collector"
)

// Register gathers host info and sends it to the server.
func (a *Agent) Register() error {
	info := collector.CollectHostInfo()
	info.Hostname = a.Config.Hostname

	payload, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("json marshal failed: %w", err)
	}

	var resp *http.Response
	var reqErr error

	url := fmt.Sprintf("%s/api/v1/agent/register", a.Config.BaseURL)

	for attempt := range a.RetryConfig.MaxAttempts {
		req, _ := http.NewRequestWithContext(a.ctx, http.MethodPost, url, bytes.NewReader(payload))
		a.setHeaders(req)
		req.Header.Del("Content-Encoding")

		resp, reqErr = a.Client.Do(req)
		if reqErr == nil && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated) {
			break
		}

		if resp != nil {
			resp.Body.Close()
		}

		if attempt < a.RetryConfig.MaxAttempts-1 {
			time.Sleep(a.RetryConfig.Delay(attempt))
		}
	}

	if reqErr != nil {
		return fmt.Errorf("network error during registration: %w", reqErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("server rejected registration: %s", resp.Status)
	}

	return nil
}
