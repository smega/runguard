package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	defaultVersionsAPI  = "https://runguard.smega.eu/api/public/cli/versions"
	defaultChannel      = "stable"
	defaultDownloadBase = "https://runguard-download.smega.eu"
)

type versionsAPIResponse struct {
	DownloadBase string   `json:"download_base"`
	Latest       string   `json:"latest"`
	Stable       string   `json:"stable"`
	RC           string   `json:"rc"`
	Nightly      string   `json:"nightly"`
	Versions     []string `json:"versions"`

	Arch      string `json:"arch"`
	Channel   string `json:"channel"`
	Version   string `json:"version"`
	BinaryURL string `json:"binary"`
	BinaryMD5 string `json:"binary_md5"`
}

func SelfUpdate(args []string) error {
	fs := flag.NewFlagSet("self-update", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // suppress default usage on parse errors

	versionFlag := fs.String("version", "", "Install this specific version instead of channel")
	channelFlag := fs.String("channel", defaultChannel, "Channel to use (latest|stable|rc|nightly)")
	baseURLFlag := fs.String("download-base", "", "Override download base URL")
	apiURLFlag := fs.String("api-url", defaultVersionsAPI, "Versions API URL (for testing)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	goos := runtime.GOOS
	goarch := runtime.GOARCH
	archParam := fmt.Sprintf("%s-%s", goos, goarch)

	targetVersion := strings.TrimSpace(*versionFlag)
	var downloadBase string
	var binaryURL string
	var md5URL string

	if targetVersion == "" {
		// Use channel + arch to pick version + binary
		resp, err := fetchVersions(*apiURLFlag, archParam, *channelFlag)
		if err != nil {
			return err
		}

		if resp.Version == "" {
			return fmt.Errorf("versions API did not return a version for arch=%s channel=%s", archParam, *channelFlag)
		}

		targetVersion = resp.Version

		if *baseURLFlag != "" {
			downloadBase = strings.TrimRight(*baseURLFlag, "/")
		} else if resp.DownloadBase != "" {
			downloadBase = strings.TrimRight(resp.DownloadBase, "/")
		} else {
			downloadBase = strings.TrimRight(defaultDownloadBase, "/")
		}

		if resp.BinaryURL != "" && *baseURLFlag == "" {
			binaryURL = resp.BinaryURL
			md5URL = resp.BinaryMD5
		} else {
			binaryURL, md5URL = composeBinaryURL(downloadBase, goos, goarch, targetVersion)
		}
	} else {
		// explicit version; only use base URL info
		if *baseURLFlag != "" {
			downloadBase = strings.TrimRight(*baseURLFlag, "/")
		} else {
			downloadBase = strings.TrimRight(defaultDownloadBase, "/")
		}
		binaryURL, md5URL = composeBinaryURL(downloadBase, goos, goarch, targetVersion)
	}

	if Version != "" && Version == targetVersion {
		fmt.Printf("runguard is already at version %s\n", targetVersion)
		return nil
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

	// Optional MD5 verification
	if md5URL != "" {
		if err := verifyMD5(md5URL, tmpPath); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: MD5 verification failed: %v\n", err)
			// non-fatal, continue
		} else {
			fmt.Println("    MD5 checksum OK.")
		}
	}

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

func fetchVersions(apiURL, arch, channel string) (*versionsAPIResponse, error) {
	u, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("invalid api-url: %w", err)
	}
	q := u.Query()
	if arch != "" {
		q.Set("arch", arch)
	}
	if channel != "" {
		q.Set("channel", channel)
	}
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String()) // #nosec G107 (URL comes from config)
	if err != nil {
		return nil, fmt.Errorf("GET versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("versions API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var v versionsAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, fmt.Errorf("decode versions response: %w", err)
	}
	return &v, nil
}

func composeBinaryURL(downloadBase, goos, goarch, version string) (string, string) {
	base := strings.TrimRight(downloadBase, "/")
	bin := fmt.Sprintf("runguard-%s-%s-%s", goos, goarch, version)
	url := base + "/" + version + "/" + bin
	return url, url + ".md5"
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

func verifyMD5(md5URL, filePath string) error {
	resp, err := http.Get(md5URL) // #nosec G107
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("md5 URL returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if err != nil {
		return err
	}
	fields := strings.Fields(string(body))
	if len(fields) == 0 {
		return fmt.Errorf("empty md5 file")
	}
	expected := strings.TrimSpace(fields[0])

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := md5.New() // fine for integrity check
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actual := hex.EncodeToString(h.Sum(nil))

	if !strings.EqualFold(expected, actual) {
		return fmt.Errorf("expected %s, got %s", expected, actual)
	}
	return nil
}
