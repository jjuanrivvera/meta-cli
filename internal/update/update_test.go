package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyVerifiedUpdate(t *testing.T) {
	archive := tarArchive(t, "meta", []byte("new-binary"))
	sum := sha256.Sum256(archive)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/jjuanrivvera/meta-cli/releases/latest":
			_ = json.NewEncoder(writer).Encode(Release{TagName: "v1.2.3", Assets: []Asset{{Name: "meta-cli_v1.2.3_linux_amd64.tar.gz", URL: server.URL + "/archive"}, {Name: "checksums.txt", URL: server.URL + "/checksums"}}})
		case "/archive":
			_, _ = writer.Write(archive)
		case "/checksums":
			fmt.Fprintf(writer, "%s  meta-cli_v1.2.3_linux_amd64.tar.gz\n", hex.EncodeToString(sum[:]))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)
	executablePath := filepath.Join(t.TempDir(), "meta")
	require.NoError(t, os.WriteFile(executablePath, []byte("old-binary"), 0o700))
	updater := New(server.Client())
	updater.APIBase = server.URL
	updater.GOOS = "linux"
	updater.GOARCH = "amd64"
	version, err := updater.Apply(context.TODO(), executablePath)
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", version)
	raw, err := os.ReadFile(executablePath)
	require.NoError(t, err)
	assert.Equal(t, "new-binary", string(raw))
	backup, err := os.ReadFile(executablePath + ".bak")
	require.NoError(t, err)
	assert.Equal(t, "old-binary", string(backup))
}

func TestUpdateValidationFailures(t *testing.T) {
	updater := New(nil)
	_, _, err := updater.findAssets(nil)
	assert.Error(t, err)
	assert.Error(t, updater.validateDownloadURL("http://example.com/file"))
	assert.Error(t, updater.validateDownloadURL("https://evil.example/file"))
	assert.NoError(t, updater.validateDownloadURL("https://github.com/file"))
	assert.Error(t, verifyChecksum("archive", []byte("bad"), []byte("00 archive")))
	assert.Error(t, verifyChecksum("missing", []byte("bad"), []byte("00 archive")))
	_, err = extractBinary("bad.tar.gz", []byte("bad"), "meta")
	assert.Error(t, err)
}

func tarArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var output bytes.Buffer
	compressed := gzip.NewWriter(&output)
	archive := tar.NewWriter(compressed)
	require.NoError(t, archive.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}))
	_, err := io.Copy(archive, bytes.NewReader(content))
	require.NoError(t, err)
	require.NoError(t, archive.Close())
	require.NoError(t, compressed.Close())
	return output.Bytes()
}
