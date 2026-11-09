package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// defaultReleasesRepo is a placeholder — override at build time with
// -ldflags "-X main.defaultReleasesRepo=owner/repo" (same mechanism as
// defaultAPIBase in main.go), or per-invocation with --repo /
// RUNGUARD_CLI_REPO. Self-update pulls releases straight from GitHub's
// public Releases API — no custom versions API or binary host needed.
var defaultReleasesRepo = "your-org/your-cli-repo"

type githubRelease struct {
	TagName string               `json:"tag_name"`
	Assets  []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func SelfUpdate(args []string) error {
	fs := flag.NewFlagSet("self-update", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // suppress default usage on parse errors

	versionFlag := fs.String("version", "", "Install this specific release tag (e.g. v0.1.3) instead of the latest")
	repoFlag := fs.String("repo", envOr("RUNGUARD_CLI_REPO", defaultReleasesRepo), "GitHub owner/repo to fetch releases from (or RUNGUARD_CLI_REPO env var)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	goos := runtime.GOOS
	goarch := runtime.GOARCH

	release, err := fetchRelease(*repoFlag, strings.TrimSpace(*versionFlag))
	if err != nil {
		return err
	}
	targetVersion := release.TagName

	if Version != "" && (Version == targetVersion || "v"+Version == targetVersion) {
		fmt.Printf("runguard is already at version %s\n", targetVersion)
		return nil
	}

	assetName := fmt.Sprintf("runguard-%s-%s", goos, goarch)
	binaryURL, sha256URL, err := findAsset(release, assetName)
	if err != nil {
		return err
	}

	fmt.Printf("==> Self-updating runguard\n")
	if Version != "" {
		fmt.Printf("    Current version : %s\n", Version)
	}
	fmt.Printf("    Target version  : %s\n", targetVersion)
	fmt.Printf("    Binary URL      : %s\n", binaryURL)

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine current executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)

	tmp, err := os.CreateTemp(exeDir, "runguard-selfupdate-*")
	if err != nil {
		return fmt.Errorf("cannot create temp file in %s: %w", exeDir, err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()

	// Download binary
	if err := httpDownload(binaryURL, tmp); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	if err := tmp.Chmod(0o755); err != nil {
		return fmt.Errorf("chmod failed: %w", err)
	}

	// SHA256 verification is fatal on mismatch — a checksum you don't
	// actually enforce isn't protecting against anything.
	if err := verifySHA256(sha256URL, tmpPath); err != nil {
		return fmt.Errorf("checksum verification failed, aborting update: %w", err)
	}
	fmt.Println("    SHA256 checksum OK.")

	// Replace binary
	if err := os.Rename(tmpPath, exePath); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("permission denied replacing %s; try 'sudo runguard self-update'", exePath)
		}
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	fmt.Printf("==> Updated runguard to %s\n", targetVersion)
	return nil
}

// fetchRelease hits GitHub's public Releases API directly — no auth, no
// custom backend involved. version == "" fetches the latest release;
// otherwise fetches that exact tag.
func fetchRelease(repo, version string) (*githubRelease, error) {
	repo = strings.Trim(repo, "/")
	if repo == "" {
		return nil, fmt.Errorf("--repo or RUNGUARD_CLI_REPO is required (e.g. smega/runguard-cli)")
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	if version != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", repo, version)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("GitHub API returned %d for %s: %s", resp.StatusCode, url, strings.TrimSpace(string(body)))
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode GitHub release response: %w", err)
	}
	return &rel, nil
}

// findAsset locates the binary and its .sha256 sibling among a release's
// assets, by the exact name release.yml publishes them under
// (runguard-{goos}-{goarch}, no version suffix — the tag carries that).
func findAsset(release *githubRelease, name string) (binaryURL, sha256URL string, err error) {
	for _, a := range release.Assets {
		switch a.Name {
		case name:
			binaryURL = a.BrowserDownloadURL
		case name + ".sha256":
			sha256URL = a.BrowserDownloadURL
		}
	}
	if binaryURL == "" {
		return "", "", fmt.Errorf("release %s has no asset named %q", release.TagName, name)
	}
	if sha256URL == "" {
		return "", "", fmt.Errorf("refusing to install: no %s.sha256 asset found for release %s", name, release.TagName)
	}
	return binaryURL, sha256URL, nil
}

func httpDownload(url string, w io.Writer) error {
	resp, err := http.Get(url) // #nosec G107
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("GET %s: status %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

func verifySHA256(sha256URL, filePath string) error {
	resp, err := http.Get(sha256URL) // #nosec G107
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sha256 URL returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if err != nil {
		return err
	}
	fields := strings.Fields(string(body))
	if len(fields) == 0 {
		return fmt.Errorf("empty sha256 file")
	}
	expected := strings.TrimSpace(fields[0])

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actual := hex.EncodeToString(h.Sum(nil))

	if !strings.EqualFold(expected, actual) {
		return fmt.Errorf("expected %s, got %s", expected, actual)
	}
	return nil
}
