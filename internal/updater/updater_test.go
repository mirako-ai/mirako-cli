package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{name: "full without v", input: "1.2.3", expected: "v1.2.3"},
		{name: "full with v", input: "v1.2.3", expected: "v1.2.3"},
		{name: "minor version", input: "1.2", expected: "v1.2.0"},
		{name: "major version", input: "v1", expected: "v1.0.0"},
		{name: "prerelease", input: "1.2.3-beta.1", expected: "v1.2.3-beta.1"},
		{name: "dev", input: "dev", wantErr: true},
		{name: "invalid", input: "not-a-version", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := NormalizeVersion(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		latest   string
		expected bool
	}{
		{name: "newer patch", current: "1.2.1", latest: "v1.2.2", expected: true},
		{name: "same", current: "v1.2.1", latest: "1.2.1", expected: false},
		{name: "older", current: "1.2.2", latest: "1.2.1", expected: false},
		{name: "minor normalized", current: "1.1", latest: "1.1.1", expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := IsNewerVersion(tt.current, tt.latest)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestAssetNameFor(t *testing.T) {
	tests := []struct {
		name       string
		goos       string
		goarch     string
		assetName  string
		binaryName string
		archiveExt string
		wantErr    bool
	}{
		{name: "darwin arm64", goos: "darwin", goarch: "arm64", assetName: "mirako-cli_Darwin_arm64.tar.gz", binaryName: "mirako", archiveExt: "tar.gz"},
		{name: "linux amd64", goos: "linux", goarch: "amd64", assetName: "mirako-cli_Linux_x86_64.tar.gz", binaryName: "mirako", archiveExt: "tar.gz"},
		{name: "windows amd64", goos: "windows", goarch: "amd64", assetName: "mirako-cli_Windows_x86_64.zip", binaryName: "mirako.exe", archiveExt: "zip"},
		{name: "unsupported os", goos: "freebsd", goarch: "amd64", wantErr: true},
		{name: "unsupported arch", goos: "linux", goarch: "386", wantErr: true},
		{name: "windows arm64", goos: "windows", goarch: "arm64", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assetName, binaryName, archiveExt, err := AssetNameFor(tt.goos, tt.goarch)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.assetName, assetName)
			assert.Equal(t, tt.binaryName, binaryName)
			assert.Equal(t, tt.archiveExt, archiveExt)
		})
	}
}

func TestCheckForUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/latest", r.URL.Path)
		_ = json.NewEncoder(w).Encode(Release{TagName: "v1.2.2"})
	}))
	defer server.Close()

	result, err := CheckForUpdate(context.Background(), "1.2.1", Options{
		HTTPClient:       server.Client(),
		LatestReleaseURL: server.URL + "/latest",
	})
	require.NoError(t, err)
	assert.True(t, result.Newer)
	assert.Equal(t, "1.2.1", result.CurrentVersion)
	assert.Equal(t, "1.2.2", result.LatestVersion)
}

func TestCheckForUpdateAlreadyLatest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Release{TagName: "v1.2.1"})
	}))
	defer server.Close()

	result, err := CheckForUpdate(context.Background(), "1.2.1", Options{
		HTTPClient:       server.Client(),
		LatestReleaseURL: server.URL,
	})
	require.NoError(t, err)
	assert.False(t, result.Newer)
}

func TestCheckForUpdateDevelopmentBuild(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	_, err := CheckForUpdate(context.Background(), "dev", Options{
		HTTPClient:       server.Client(),
		LatestReleaseURL: server.URL,
	})
	require.ErrorIs(t, err, ErrDevelopmentBuild)
	assert.False(t, called)
}

func TestInstallLatest(t *testing.T) {
	assetName, _, _, err := AssetNameFor("linux", "amd64")
	require.NoError(t, err)

	archive := tarGzArchive(t, "mirako", []byte("#!/bin/sh\necho new\n"))
	checksum := sha256.Sum256(archive)
	checksums := []byte(fmt.Sprintf("%s  %s\n", hex.EncodeToString(checksum[:]), assetName))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			_ = json.NewEncoder(w).Encode(Release{
				TagName: "v1.2.0",
				Assets: []Asset{
					{Name: assetName, BrowserDownloadURL: "http://" + r.Host + "/" + assetName},
					{Name: "checksums.txt", BrowserDownloadURL: "http://" + r.Host + "/checksums.txt"},
				},
			})
		case "/" + assetName:
			_, _ = w.Write(archive)
		case "/checksums.txt":
			_, _ = w.Write(checksums)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	target := filepath.Join(dir, "mirako")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0755))

	var output bytes.Buffer
	result, err := InstallLatest(context.Background(), "1.1.0", Options{
		HTTPClient:       server.Client(),
		LatestReleaseURL: server.URL + "/latest",
		GOOS:             "linux",
		GOARCH:           "amd64",
		ExecutablePath:   target,
	}, &output)
	require.NoError(t, err)
	assert.True(t, result.Newer)
	assert.Contains(t, output.String(), "Updated Mirako CLI to 1.2.0")

	installed, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\necho new\n", string(installed))
}

func TestInstallLatestChecksumMismatch(t *testing.T) {
	assetName, _, _, err := AssetNameFor("linux", "amd64")
	require.NoError(t, err)

	archive := tarGzArchive(t, "mirako", []byte("new"))
	checksums := []byte(fmt.Sprintf("%064x  %s\n", 0, assetName))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			_ = json.NewEncoder(w).Encode(Release{
				TagName: "v1.2.0",
				Assets: []Asset{
					{Name: assetName, BrowserDownloadURL: "http://" + r.Host + "/" + assetName},
					{Name: "checksums.txt", BrowserDownloadURL: "http://" + r.Host + "/checksums.txt"},
				},
			})
		case "/" + assetName:
			_, _ = w.Write(archive)
		case "/checksums.txt":
			_, _ = w.Write(checksums)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "mirako")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0755))

	_, err = InstallLatest(context.Background(), "1.1.0", Options{
		HTTPClient:       server.Client(),
		LatestReleaseURL: server.URL + "/latest",
		GOOS:             "linux",
		GOARCH:           "amd64",
		ExecutablePath:   target,
	}, ioDiscard{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum verification failed")

	installed, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, "old", string(installed))
}

func TestIsHomebrewManaged(t *testing.T) {
	assert.True(t, IsHomebrewManaged("/opt/homebrew/Cellar/mirako/1.2.1/bin/mirako"))
	assert.True(t, IsHomebrewManaged("/usr/local/Cellar/mirako-cli/1.2.1/bin/mirako"))
	assert.False(t, IsHomebrewManaged("/Users/me/.mirako/bin/mirako"))
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func tarGzArchive(t *testing.T, name string, contents []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	require.NoError(t, tarWriter.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0755,
		Size: int64(len(contents)),
	}))
	_, err := tarWriter.Write(contents)
	require.NoError(t, err)
	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())

	return buf.Bytes()
}
