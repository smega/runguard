// cmd/runguard/main.go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Version can be overridden at build-time with:
//
//	go build -ldflags "-X main.Version=v0.1.0"
//
// var Version = "dev"
var Version = "0.1.3" // override via -ldflags at build time

type heartbeatPayload struct {
	Status     string `json:"status"`
	Output     string `json:"output,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	ExitCode   int    `json:"exit_code,omitempty"`
	Hostname   string `json:"hostname,omitempty"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "self-update" {
		if err := SelfUpdate(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "self-update error:", err)
			os.Exit(1)
		}
		return
	}
	// if len(os.Args) < 2 {
	// 	printUsage()
	// 	os.Exit(1)
	// }

	switch os.Args[1] {
	case "wrap":
		runWrap(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Println(Version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `RunGuard CLI

Subcommands:

  runguard wrap [flags] -- command [args...]
      Run a command, capture its output, and post a heartbeat to RunGuard.

  runguard version
      Print CLI version.

Examples:

  runguard wrap --key=RUNGUARD_KEY -- /opt/scripts/orders.sh

Common flags for "wrap":

  --key=RUNGUARD_KEY        Required. Job heartbeat key as shown in RunGuard.
  --endpoint=URL            Optional. Full heartbeat endpoint.
                            If not set, defaults to https://runguard.smega.eu/hb/KEY
  --timeout=SECONDS         HTTP timeout in seconds (default 10).
  --truncate=BYTES          Max output snippet bytes (default 4000).
`)
}

// runWrap implements: runguard wrap --key KEY [--endpoint URL] -- cmd args...
func runWrap(args []string) {
	fs := flag.NewFlagSet("wrap", flag.ExitOnError)
	key := fs.String("key", "", "RunGuard heartbeat key (required if --endpoint not set)")
	endpoint := fs.String("endpoint", "", "Full heartbeat endpoint URL (optional)")
	timeoutSeconds := fs.Int("timeout", 10, "HTTP timeout in seconds")
	truncateBytes := fs.Int("truncate", 4000, "Max number of bytes from the end of job output to send")

	// Allow endpoint/key from environment too
	envKey := os.Getenv("RUNGUARD_KEY")
	envEndpoint := os.Getenv("RUNGUARD_ENDPOINT")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: runguard wrap [flags] -- command [args...]\n\n")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	// After flags, we expect a "--" then command
	cmdArgs := fs.Args()
	if len(cmdArgs) == 0 {
		fmt.Fprintln(os.Stderr, "No command specified. Use: runguard wrap --key=KEY -- my-command ...")
		os.Exit(1)
	}

	// If the first arg is "--", strip it
	if cmdArgs[0] == "--" {
		cmdArgs = cmdArgs[1:]
	}
	if len(cmdArgs) == 0 {
		fmt.Fprintln(os.Stderr, "No command specified after --.")
		os.Exit(1)
	}

	// Resolve key/endpoint
	finalEndpoint := strings.TrimSpace(*endpoint)
	finalKey := strings.TrimSpace(*key)

	if finalEndpoint == "" && envEndpoint != "" {
		finalEndpoint = strings.TrimSpace(envEndpoint)
	}
	if finalKey == "" && envKey != "" {
		finalKey = strings.TrimSpace(envKey)
	}

	if finalEndpoint == "" {
		if finalKey == "" {
			fmt.Fprintln(os.Stderr, "Error: either --endpoint or --key (or RUNGUARD_KEY / RUNGUARD_ENDPOINT) must be set.")
			os.Exit(1)
		}
		finalEndpoint = "https://runguard.smega.eu/hb/" + finalKey
	}

	// Run the command
	command := cmdArgs[0]
	commandArgs := cmdArgs[1:]

	start := time.Now()
	cmd := exec.Command(command, commandArgs...)
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			// Non-exit error (e.g. exec: not found)
			exitCode = 1
		}
	}

	status := "ok"
	if exitCode != 0 {
		status = "error"
	}

	// Prepare output snippet (from the end)
	rawOutput := combined.Bytes()
	maxBytes := *truncateBytes
	if maxBytes <= 0 {
		maxBytes = 4000
	}
	if len(rawOutput) > maxBytes {
		rawOutput = rawOutput[len(rawOutput)-maxBytes:]
	}
	outputStr := string(rawOutput)

	hostname, _ := os.Hostname()

	payload := heartbeatPayload{
		Status:     status,
		Output:     outputStr,
		DurationMs: duration.Milliseconds(),
		ExitCode:   exitCode,
		Hostname:   hostname,
	}

	if err := postHeartbeat(finalEndpoint, payload, time.Duration(*timeoutSeconds)*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to post heartbeat: %v\n", err)
		// Don't override the actual job exit code; caller cares about that
	}

	os.Exit(exitCode)
}

func postHeartbeat(endpoint string, payload heartbeatPayload, timeout time.Duration) error {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "runguard-cli/"+Version)

	client := &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post heartbeat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("heartbeat non-2xx: %s", resp.Status)
	}

	return nil
}

type CaptureMode string

const (
	CaptureNone   CaptureMode = "none"
	CaptureStdout CaptureMode = "stdout"
	CaptureStderr CaptureMode = "stderr"
	CaptureBoth   CaptureMode = "both"
)

type WrapConfig struct {
	JobID        string
	Env          string
	Tags         []string
	Capture      CaptureMode
	TailBytes    int
	APIBaseURL   string
	AgentVersion string
}
type HeartbeatRequest struct {
	JobID        string    `json:"job_id"`
	Ts           time.Time `json:"ts"`
	Status       string    `json:"status"`
	ExitCode     *int      `json:"exit_code,omitempty"`
	DurationMs   int64     `json:"duration_ms"`
	Env          string    `json:"env,omitempty"`
	Tags         []string  `json:"tags,omitempty"`
	AgentVersion string    `json:"agent_version,omitempty"`

	StdoutTail string `json:"stdout_tail,omitempty"`
	StderrTail string `json:"stderr_tail,omitempty"`
}
