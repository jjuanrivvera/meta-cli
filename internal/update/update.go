package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const maxArtifactSize = 256 << 20

type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type Updater struct {
	Client  *http.Client
	APIBase string
	Repo    string
	Binary  string
	GOOS    string
	GOARCH  string
}

func New(client *http.Client) *Updater {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &Updater{Client: client, APIBase: "https://api.github.com", Repo: "jjuanrivvera/meta-cli", Binary: "metactl", GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
}

func (updater *Updater) Latest(ctx context.Context) (*Release, error) {
	endpoint := strings.TrimSuffix(updater.APIBase, "/") + "/repos/" + updater.Repo + "/releases/latest"
	raw, err := updater.download(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}
	var release Release
	if err := json.Unmarshal(raw, &release); err != nil {
		return nil, fmt.Errorf("decode latest release: %w", err)
	}
	if release.TagName == "" {
		return nil, fmt.Errorf("latest release response omitted tag_name")
	}
	return &release, nil
}

func (updater *Updater) Apply(ctx context.Context, executablePath string) (string, error) {
	release, err := updater.Latest(ctx)
	if err != nil {
		return "", err
	}
	archiveAsset, checksumAsset, err := updater.findAssets(release.Assets)
	if err != nil {
		return "", err
	}
	archive, err := updater.download(ctx, archiveAsset.URL)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", archiveAsset.Name, err)
	}
	checksums, err := updater.download(ctx, checksumAsset.URL)
	if err != nil {
		return "", fmt.Errorf("download checksums: %w", err)
	}
	if err := verifyChecksum(archiveAsset.Name, archive, checksums); err != nil {
		return "", err
	}
	binary, err := extractBinary(archiveAsset.Name, archive, updater.Binary)
	if err != nil {
		return "", err
	}
	if executablePath == "" {
		executablePath, err = os.Executable()
		if err != nil {
			return "", fmt.Errorf("locate executable: %w", err)
		}
	}
	if err := replaceExecutable(executablePath, binary); err != nil {
		return "", err
	}
	return release.TagName, nil
}

func (updater *Updater) findAssets(assets []Asset) (Asset, Asset, error) {
	var archive, checksums Asset
	osAliases := []string{updater.GOOS}
	archAliases := []string{updater.GOARCH}
	if updater.GOARCH == "amd64" {
		archAliases = append(archAliases, "x86_64")
	}
	extensions := []string{".tar.gz"}
	if updater.GOOS == "windows" {
		extensions = []string{".zip"}
	}
	for _, asset := range assets {
		lower := strings.ToLower(asset.Name)
		if lower == "checksums.txt" {
			checksums = asset
			continue
		}
		if containsAny(lower, osAliases) && containsAny(lower, archAliases) && hasAnySuffix(lower, extensions) {
			archive = asset
		}
	}
	if archive.URL == "" || checksums.URL == "" {
		return Asset{}, Asset{}, fmt.Errorf("release lacks an archive for %s/%s or checksums.txt", updater.GOOS, updater.GOARCH)
	}
	return archive, checksums, nil
}

func (updater *Updater) download(ctx context.Context, rawURL string) ([]byte, error) {
	if err := updater.validateDownloadURL(rawURL); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := updater.Client.Do(request) // #nosec G107 -- URL scheme and host are restricted below
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %s", response.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxArtifactSize+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxArtifactSize {
		return nil, fmt.Errorf("download exceeds %d bytes", maxArtifactSize)
	}
	return raw, nil
}

func (updater *Updater) validateDownloadURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return fmt.Errorf("invalid download URL")
	}
	host := parsed.Hostname()
	if parsed.Scheme == "http" {
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return fmt.Errorf("plain HTTP download URL is not loopback")
		}
		return nil
	}
	if host != "api.github.com" && host != "github.com" && !strings.HasSuffix(host, ".githubusercontent.com") {
		return fmt.Errorf("untrusted download host %q", host)
	}
	return nil
}

func verifyChecksum(name string, archive, checksums []byte) error {
	want := ""
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.TrimPrefix(fields[len(fields)-1], "*") == name {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("checksums.txt has no entry for %s", name)
	}
	sum := sha256.Sum256(archive)
	if !strings.EqualFold(want, hex.EncodeToString(sum[:])) {
		return fmt.Errorf("checksum mismatch for %s", name)
	}
	return nil
}

func extractBinary(archiveName string, archive []byte, binaryName string) ([]byte, error) {
	if strings.HasSuffix(strings.ToLower(archiveName), ".zip") {
		reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, err
		}
		for _, file := range reader.File {
			if filepath.Base(file.Name) == binaryName || filepath.Base(file.Name) == binaryName+".exe" {
				stream, err := file.Open()
				if err != nil {
					return nil, err
				}
				defer func() { _ = stream.Close() }()
				return io.ReadAll(io.LimitReader(stream, maxArtifactSize))
			}
		}
	} else {
		compressed, err := gzip.NewReader(bytes.NewReader(archive))
		if err != nil {
			return nil, err
		}
		defer func() { _ = compressed.Close() }()
		reader := tar.NewReader(compressed)
		for {
			header, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if filepath.Base(header.Name) == binaryName {
				return io.ReadAll(io.LimitReader(reader, maxArtifactSize))
			}
		}
	}
	return nil, fmt.Errorf("archive does not contain %s", binaryName)
}

func replaceExecutable(executablePath string, binary []byte) error {
	info, err := os.Stat(executablePath)
	if err != nil {
		return fmt.Errorf("stat executable: %w", err)
	}
	directory := filepath.Dir(executablePath)
	temporary, err := os.CreateTemp(directory, ".metactl-update-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if _, err := temporary.Write(binary); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Chmod(info.Mode().Perm()); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	backupPath := executablePath + ".bak"
	_ = os.Remove(backupPath)
	if err := os.Rename(executablePath, backupPath); err != nil {
		return fmt.Errorf("backup executable: %w", err)
	}
	if err := os.Rename(temporaryPath, executablePath); err != nil {
		_ = os.Rename(backupPath, executablePath)
		return fmt.Errorf("replace executable: %w", err)
	}
	return nil
}

func containsAny(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, strings.ToLower(candidate)) {
			return true
		}
	}
	return false
}
func hasAnySuffix(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if strings.HasSuffix(value, candidate) {
			return true
		}
	}
	return false
}
