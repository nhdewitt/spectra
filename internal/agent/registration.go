package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nhdewitt/spectra/internal/hostinfo"
	"github.com/nhdewitt/spectra/internal/protocol"
)

// Register gathers host info and sends it to the server.
func (a *Agent) Register(ctx context.Context) error {
	info := hostinfo.CollectHostInfo()
	info.Hostname = a.Config.Hostname

	regReq := protocol.RegisterRequest{
		Token: a.Config.RegistrationToken,
		Info:  info,
	}

	payload, err := json.Marshal(regReq)
	if err != nil {
		return fmt.Errorf("json marshal failed: %w", err)
	}

	var httpResp *http.Response
	var reqErr error

	url := fmt.Sprintf("%s/api/v1/agent/register", a.Config.BaseURL)

	for attempt := range a.RetryConfig.MaxAttempts {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		a.setHeaders(req)
		req.Header.Del("Content-Encoding")

		httpResp, reqErr = a.Client.Do(req)
		if reqErr == nil && (httpResp.StatusCode == http.StatusOK || httpResp.StatusCode == http.StatusCreated) {
			break
		}

		if httpResp != nil {
			httpResp.Body.Close()
		}

		if attempt < a.RetryConfig.MaxAttempts-1 {
			time.Sleep(a.RetryConfig.Delay(attempt))
		}
	}

	if reqErr != nil {
		return fmt.Errorf("network error during registration: %w", reqErr)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusCreated {
		return fmt.Errorf("server rejected registration: %s", httpResp.Status)
	}

	var resp protocol.RegisterResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	a.Identity = Identity{
		ID:     resp.AgentID,
		Secret: resp.Secret,
	}

	if err := saveIdentity(a.Identity, a.Config.IdentityPath); err != nil {
		return fmt.Errorf("saving identity: %w", err)
	}

	return nil
}
