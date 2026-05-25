package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	// DefaultLatestReleaseURL is the GitHub API endpoint used to discover the latest CLI release.
	DefaultLatestReleaseURL = "https://api.github.com/repos/mirako-ai/mirako-cli/releases/latest"
	defaultUserAgent        = "mirako-cli-updater"
)

var (
	// ErrDevelopmentBuild is returned when the current binary is not a release build.
	ErrDevelopmentBuild = errors.New("development build cannot be updated automatically")
	// ErrHomebrewManaged is returned when the binary appears to be managed by Homebrew.
	ErrHomebrewManaged = errors.New("homebrew-managed installation")
)

// Options configures update checks and installation.
type Options struct {
	HTTPClient       *http.Client
	LatestReleaseURL string
	GOOS             string
	GOARCH           string
	ExecutablePath   string
}

// Asset is a downloadable release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Release contains the release metadata needed by the updater.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// CheckResult describes the current and latest release state.
type CheckResult struct {
	CurrentVersion string
	LatestVersion  string
	LatestTag      string
	Newer          bool
	Release        Release
}

func (o Options) withDefaults() Options {
	if o.HTTPClient == nil {
		o.HTTPClient = http.DefaultClient
	}
	if o.LatestReleaseURL == "" {
		o.LatestReleaseURL = DefaultLatestReleaseURL
	}
	if o.GOOS == "" {
		o.GOOS = runtime.GOOS
	}
	if o.GOARCH == "" {
		o.GOARCH = runtime.GOARCH
	}
	return o
}

// NormalizeVersion converts release versions like "1.2" or "v1.2.3" into valid semver strings.
func NormalizeVersion(version string) (string, error) {
	v := strings.TrimSpace(version)
	if v == "" || strings.EqualFold(v, "dev") {
		return "", ErrDevelopmentBuild
	}

	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}

	coreEnd := len(v)
	if idx := strings.IndexAny(v, "-+"); idx >= 0 {
		coreEnd = idx
	}
	core := v[:coreEnd]
	suffix := v[coreEnd:]

	switch strings.Count(core, ".") {
	case 0:
		v = core + ".0.0" + suffix
	case 1:
		v = core + ".0" + suffix
	}

	if !semver.IsValid(v) {
		return "", fmt.Errorf("invalid version %q", version)
	}

	return v, nil
}

// DisplayVersion removes the leading "v" from a normalized version for user-facing output.
func DisplayVersion(version string) string {
	return strings.TrimPrefix(version, "v")
}

// IsNewerVersion reports whether latest is newer than current.
func IsNewerVersion(current, latest string) (bool, error) {
	currentNormalized, err := NormalizeVersion(current)
	if err != nil {
		return false, err
	}
	latestNormalized, err := NormalizeVersion(latest)
	if err != nil {
		return false, err
	}
	return semver.Compare(latestNormalized, currentNormalized) > 0, nil
}

// FetchLatestRelease retrieves the latest release metadata.
func FetchLatestRelease(ctx context.Context, opts Options) (Release, error) {
	opts = opts.withDefaults()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, opts.LatestReleaseURL, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", defaultUserAgent)

	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Release{}, fmt.Errorf("failed to fetch latest release: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return Release{}, fmt.Errorf("failed to decode latest release: %w", err)
	}
	if release.TagName == "" {
		return Release{}, errors.New("latest release response did not include tag_name")
	}

	return release, nil
}

// CheckForUpdate checks GitHub releases and reports whether a newer version is available.
func CheckForUpdate(ctx context.Context, currentVersion string, opts Options) (CheckResult, error) {
	currentNormalized, err := NormalizeVersion(currentVersion)
	if err != nil {
		return CheckResult{}, err
	}

	release, err := FetchLatestRelease(ctx, opts)
	if err != nil {
		return CheckResult{}, err
	}

	latestNormalized, err := NormalizeVersion(release.TagName)
	if err != nil {
		return CheckResult{}, fmt.Errorf("latest release has invalid version %q: %w", release.TagName, err)
	}

	return CheckResult{
		CurrentVersion: DisplayVersion(currentNormalized),
		LatestVersion:  DisplayVersion(latestNormalized),
		LatestTag:      release.TagName,
		Newer:          semver.Compare(latestNormalized, currentNormalized) > 0,
		Release:        release,
	}, nil
}

// AssetNameFor returns the release archive name and binary name for a platform.
func AssetNameFor(goos, goarch string) (assetName, binaryName, archiveExt string, err error) {
	var assetOS, assetArch string

	switch goos {
	case "darwin":
		assetOS = "Darwin"
		archiveExt = "tar.gz"
	case "linux":
		assetOS = "Linux"
		archiveExt = "tar.gz"
	case "windows":
		assetOS = "Windows"
		archiveExt = "zip"
	default:
		return "", "", "", fmt.Errorf("unsupported operating system: %s", goos)
	}

	switch goarch {
	case "amd64":
		assetArch = "x86_64"
	case "arm64":
		assetArch = "arm64"
	default:
		return "", "", "", fmt.Errorf("unsupported architecture: %s", goarch)
	}

	if goos == "windows" && goarch == "arm64" {
		return "", "", "", errors.New("Windows arm64 is not published yet")
	}

	binaryName = "mirako"
	if goos == "windows" {
		binaryName += ".exe"
	}

	assetName = fmt.Sprintf("mirako-cli_%s_%s.%s", assetOS, assetArch, archiveExt)
	return assetName, binaryName, archiveExt, nil
}

func findAsset(release Release, name string) (Asset, bool) {
	for _, asset := range release.Assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return Asset{}, false
}

// InstallLatest downloads, verifies, extracts, and replaces the current executable with the latest release.
func InstallLatest(ctx context.Context, currentVersion string, opts Options, out io.Writer) (CheckResult, error) {
	if out == nil {
		out = io.Discard
	}
	opts = opts.withDefaults()

	result, err := CheckForUpdate(ctx, currentVersion, opts)
	if err != nil {
		return CheckResult{}, err
	}
	if !result.Newer {
		return result, nil
	}

	executablePath, err := resolveExecutablePath(opts.ExecutablePath)
	if err != nil {
		return result, err
	}
	if IsHomebrewManaged(executablePath) {
		return result, fmt.Errorf("%w: %s", ErrHomebrewManaged, executablePath)
	}

	assetName, binaryName, archiveExt, err := AssetNameFor(opts.GOOS, opts.GOARCH)
	if err != nil {
		return result, err
	}

	asset, ok := findAsset(result.Release, assetName)
	if !ok {
		return result, fmt.Errorf("release %s does not include asset %s", result.LatestTag, assetName)
	}
	checksumsAsset, ok := findAsset(result.Release, "checksums.txt")
	if !ok {
		return result, fmt.Errorf("release %s does not include checksums.txt", result.LatestTag)
	}

	tmpDir, err := os.MkdirTemp("", "mirako-update-*")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, assetName)
	checksumsPath := filepath.Join(tmpDir, "checksums.txt")
	extractedPath := filepath.Join(tmpDir, binaryName)

	fmt.Fprintf(out, "Current version: %s\n", result.CurrentVersion)
	fmt.Fprintf(out, "Latest version: %s\n", result.LatestVersion)
	fmt.Fprintf(out, "Downloading %s...\n", assetName)
	if err := downloadFile(ctx, opts.HTTPClient, asset.BrowserDownloadURL, archivePath); err != nil {
		return result, err
	}
	if err := downloadFile(ctx, opts.HTTPClient, checksumsAsset.BrowserDownloadURL, checksumsPath); err != nil {
		return result, err
	}

	fmt.Fprintln(out, "Verifying checksum...")
	if err := verifyChecksumFile(archivePath, checksumsPath, assetName); err != nil {
		return result, err
	}

	if err := extractBinary(archivePath, archiveExt, binaryName, extractedPath); err != nil {
		return result, err
	}

	fmt.Fprintf(out, "Installing to %s...\n", executablePath)
	if err := replaceExecutable(extractedPath, executablePath); err != nil {
		return result, err
	}

	fmt.Fprintf(out, "Updated Mirako CLI to %s\n", result.LatestVersion)
	return result, nil
}

// IsHomebrewManaged reports whether the executable path appears to be under a Homebrew Cellar.
func IsHomebrewManaged(executablePath string) bool {
	path := filepath.ToSlash(executablePath)
	return strings.Contains(path, "/Cellar/mirako/") || strings.Contains(path, "/Cellar/mirako-cli/")
}

func resolveExecutablePath(path string) (string, error) {
	if path == "" {
		executablePath, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("failed to determine current executable path: %w", err)
		}
		path = executablePath
	}

	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return filepath.Abs(path)
}

func downloadFile(ctx context.Context, client *http.Client, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", defaultUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("failed to download %s: HTTP %d %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

func verifyChecksumFile(archivePath, checksumsPath, assetName string) error {
	contents, err := os.ReadFile(checksumsPath)
	if err != nil {
		return err
	}

	expected, err := expectedChecksum(contents, assetName)
	if err != nil {
		return err
	}

	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum verification failed for %s", assetName)
	}
	return nil
}

func expectedChecksum(contents []byte, assetName string) (string, error) {
	for _, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[len(fields)-1] == assetName {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("checksum for %s was not found", assetName)
}

func extractBinary(archivePath, archiveExt, binaryName, destPath string) error {
	switch archiveExt {
	case "tar.gz":
		return extractBinaryFromTarGz(archivePath, binaryName, destPath)
	case "zip":
		return extractBinaryFromZip(archivePath, binaryName, destPath)
	default:
		return fmt.Errorf("unsupported archive format: %s", archiveExt)
	}
}

func extractBinaryFromTarGz(archivePath, binaryName, destPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != binaryName {
			continue
		}
		return writeExecutable(destPath, tarReader)
	}

	return fmt.Errorf("archive did not contain %s", binaryName)
}

func extractBinaryFromZip(archivePath, binaryName, destPath string) error {
	zipReader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zipReader.Close()

	for _, file := range zipReader.File {
		if file.FileInfo().IsDir() || filepath.Base(file.Name) != binaryName {
			continue
		}

		reader, err := file.Open()
		if err != nil {
			return err
		}
		defer reader.Close()
		return writeExecutable(destPath, reader)
	}

	return fmt.Errorf("archive did not contain %s", binaryName)
}

func writeExecutable(path string, reader io.Reader) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return err
	}
	return file.Chmod(0755)
}

func replaceExecutable(sourcePath, targetPath string) error {
	dir := filepath.Dir(targetPath)
	tmpFile, err := os.CreateTemp(dir, ".mirako-update-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s: %w", dir, err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	source, err := os.Open(sourcePath)
	if err != nil {
		_ = tmpFile.Close()
		return err
	}
	defer source.Close()

	if _, err := io.Copy(tmpFile, source); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Chmod(0755); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to replace %s: %w", targetPath, err)
	}
	return nil
}

// CheckWithTimeout is a convenience wrapper for short, best-effort update checks.
func CheckWithTimeout(currentVersion string, timeout time.Duration) (CheckResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return CheckForUpdate(ctx, currentVersion, Options{})
}
