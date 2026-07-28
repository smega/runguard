// cli_versions.go
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// CLIVersionsResponse is returned by the public CLI versions endpoint.
// Versions are discovered live from the download bucket; channel labels
// (stable/rc/nightly) and the default channel come from config.yml so they
// can be updated without a rebuild/redeploy.
type CLIVersionsResponse struct {
	DownloadBase string   `json:"download_base"`
	Latest       string   `json:"latest"`
	Stable       string   `json:"stable,omitempty"`
	RC           string   `json:"rc,omitempty"`
	Nightly      string   `json:"nightly,omitempty"`
	Versions     []string `json:"versions"`

	// Arch-aware extras (populated when ?arch= is provided)
	Arch         string `json:"arch,omitempty"`
	Channel      string `json:"channel,omitempty"`
	Version      string `json:"version,omitempty"`
	BinaryURL    string `json:"binary,omitempty"`
	BinarySHA256 string `json:"binary_sha256,omitempty"`
}

func (a *App) handleCLIListVersions(c *gin.Context) {
	downloadBase := a.cfg.CLI.DownloadBase

	versions, err := discoverCLIVersions(downloadBase)
	if err != nil {
		log.Printf("[cli] discover versions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to list CLI versions",
		})
		return
	}

	latest := ""
	if len(versions) > 0 {
		latest = versions[len(versions)-1]
	}

	resp := CLIVersionsResponse{
		DownloadBase: downloadBase,
		Latest:       latest,
		Stable:       a.cfg.CLI.Stable,
		RC:           a.cfg.CLI.RC,
		Nightly:      a.cfg.CLI.Nightly,
		Versions:     versions,
	}

	arch := c.Query("arch")
	channel := c.Query("channel")
	if channel == "" {
		channel = a.cfg.CLI.DefaultChannel
	}

	if arch != "" {
		resp.Arch = arch
		resp.Channel = channel

		version := pickChannelVersion(channel, resp)
		if version != "" {
			resp.Version = version

			base := strings.TrimRight(resp.DownloadBase, "/")
			goos, goarch := splitArch(arch) // "linux-amd64" -> "linux","amd64"

			if goos != "" && goarch != "" {
				bin := "runguard-" + goos + "-" + goarch + "-" + version
				url := base + "/" + version + "/" + bin
				resp.BinaryURL = url
				resp.BinarySHA256 = url + ".sha256"
			}
		}
	}

	c.JSON(http.StatusOK, resp)
}

func discoverCLIVersions(baseURL string) ([]string, error) {
	base := strings.TrimRight(baseURL, "/") + "/"

	resp, err := http.Get(base)
	if err != nil {
		return nil, fmt.Errorf("fetch index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// Match href="0.1.1/" etc (Nginx autoindex style)
	re := regexp.MustCompile(`href="([0-9]+\.[0-9]+\.[0-9]+)/"`)
	matches := re.FindAllStringSubmatch(string(body), -1)

	if len(matches) == 0 {
		return nil, fmt.Errorf("no semver directories found at %s", base)
	}

	seen := make(map[string]struct{})
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		v := m[1]
		seen[v] = struct{}{}
	}

	var versions []string
	for v := range seen {
		versions = append(versions, v)
	}

	sort.Slice(versions, func(i, j int) bool {
		return semverLess(versions[i], versions[j])
	})

	return versions, nil
}

func semverLess(a, b string) bool {
	majA, minA, patA := parseSemver(a)
	majB, minB, patB := parseSemver(b)

	if majA != majB {
		return majA < majB
	}
	if minA != minB {
		return minA < minB
	}
	return patA < patB
}

func parseSemver(v string) (int, int, int) {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return 0, 0, 0
	}
	maj, _ := strconv.Atoi(parts[0])
	min, _ := strconv.Atoi(parts[1])
	pat, _ := strconv.Atoi(parts[2])
	return maj, min, pat
}

func pickChannelVersion(channel string, resp CLIVersionsResponse) string {
	switch channel {
	case "latest":
		if resp.Latest != "" {
			return resp.Latest
		}
	case "stable":
		if resp.Stable != "" {
			return resp.Stable
		}
	case "rc":
		if resp.RC != "" {
			return resp.RC
		}
	case "nightly":
		if resp.Nightly != "" {
			return resp.Nightly
		}
	}
	// Fallback if channel is unknown or empty: latest
	return resp.Latest
}

// "linux-amd64" -> "linux","amd64"
func splitArch(arch string) (string, string) {
	parts := strings.SplitN(arch, "-", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
