// register.go — talks to RunGuard's /api/agent/register, same contract the
// runguard CLI's "register" subcommand uses (see backend/agent.go and
// backend/cmd/runguard/main.go in the main repo). Kept deliberately separate
// from any shared Go module with the backend, same reasoning as the CLI:
// one HTTP contract, no compile-time coupling.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type registerRequest struct {
	Name     string `json:"name"`
	CronExpr string `json:"cron_expr,omitempty"`
	Severity string `json:"severity,omitempty"`
}

type registerResponse struct {
	JobID        string `json:"job_id"`
	Name         string `json:"name"`
	Key          string `json:"key"`
	HeartbeatURL string `json:"heartbeat_url"`
	Created      bool   `json:"created"`
	Error        string `json:"error,omitempty"`
}

type runguardClient struct {
	apiBase    string
	agentToken string
	httpClient *http.Client
}

func newRunguardClient(apiBase, agentToken string) *runguardClient {
	return &runguardClient{
		apiBase:    strings.TrimRight(apiBase, "/"),
		agentToken: agentToken,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// register finds-or-creates a RunGuard job by name, matching the given cron
// expression. Idempotent server-side: calling this repeatedly with the same
// name is safe and always returns the same job/key.
func (rc *runguardClient) register(ctx context.Context, name, cronExpr, severity string) (*registerResponse, error) {
	body, err := json.Marshal(registerRequest{Name: name, CronExpr: cronExpr, Severity: severity})
	if err != nil {
		return nil, fmt.Errorf("marshal register request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rc.apiBase+"/api/agent/register", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build register request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+rc.agentToken)
	req.Header.Set("User-Agent", "runguard-k8s-operator/"+version)

	resp, err := rc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("register request failed: %w", err)
	}
	defer resp.Body.Close()

	var out registerResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode register response (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := out.Error
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("register failed: %s", msg)
	}
	return &out, nil
}
