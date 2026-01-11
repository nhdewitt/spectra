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
	fmt.Println("Collecting system inventory...")

	info := collector.CollectHostInfo()
	info.Hostname = a.Config.Hostname

	payload, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("json marshal failed: %w", err)
	}

	var resp *http.Response
	var reqErr error

	url := fmt.Sprintf("%s/api/v1/agent/register", a.Config.BaseURL)

	// Request with retries
	for range 3 {
		req, _ := http.NewRequestWithContext(a.ctx, "POST", url, bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")

		resp, reqErr = a.Client.Do(req)
		if reqErr == nil && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated) {
			break
		}

		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}

	if reqErr != nil {
		return fmt.Errorf("network error during registration: %w", reqErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("server rejected registration: %s", resp.Status)
	}

	fmt.Printf("Agent Registered Successfully as '%s'", info.Hostname)
	fmt.Printf("   OS: %s %s | CPU: %s (%d Cores)\n", info.Platform, info.PlatVer, info.CPUModel, info.CPUCores)

	return nil
}
