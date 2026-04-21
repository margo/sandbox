// shared-lib/file/http_test.go
package file

import (
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/sandbox/shared-lib/http/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadFileFromRealServer(t *testing.T) {
	tempDir := t.TempDir()
	options := &DownloadOptions{
		OutputPath:     filepath.Join(tempDir, "test-file.txt"),
		CreateDirs:     true,
		OverwriteExist: true,
		Timeout:        10 * time.Second,
	}
	url := "https://raw.githubusercontent.com/docker/awesome-compose/refs/heads/master/nextcloud-redis-mariadb/compose.yaml"

	result, err := DownloadFileUsingHttp("GET", url, nil, nil, nil, options)

	require.NoError(t, err)

	// Verify file content
	content, err := os.ReadFile(result.FilePath)
	require.NoError(t, err)
	log.Println("downloaded content", string(content))
}

func TestDownloadFileUsingHttp_SuccessfulDownload(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", "11")
		w.Header().Set("ETag", `"test-etag"`)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello World"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	options := &DownloadOptions{
		OutputPath:     filepath.Join(tempDir, "test-file.txt"),
		CreateDirs:     true,
		OverwriteExist: true,
		Timeout:        10 * time.Second,
	}

	result, err := DownloadFileUsingHttp("GET", server.URL+"/test", nil, nil, nil, options)

	require.NoError(t, err)
	assert.Equal(t, int64(11), result.Size)
	assert.Equal(t, "text/plain", result.ContentType)
	assert.Equal(t, `"test-etag"`, result.ETag)
	assert.Equal(t, http.StatusOK, result.StatusCode)

	// Verify file content
	content, err := os.ReadFile(result.FilePath)
	require.NoError(t, err)
	assert.Equal(t, "Hello World", string(content))
}

func TestDownloadFileUsingHttp_AuthenticationRequired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Authenticated content"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	auth := &auth.AuthConfig{
		Type:  auth.AuthTypeBearer,
		Token: "test-token",
	}
	options := &DownloadOptions{
		OutputPath:     filepath.Join(tempDir, "auth-file.txt"),
		CreateDirs:     true,
		OverwriteExist: true,
		Timeout:        10 * time.Second,
	}

	result, err := DownloadFileUsingHttp("GET", server.URL+"/secure", auth, nil, nil, options)

	require.NoError(t, err)
	content, err := os.ReadFile(result.FilePath)
	require.NoError(t, err)
	assert.Equal(t, "Authenticated content", string(content))
}

func TestDownloadFileUsingHttp_FileSizeLimitExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "2048") // 2KB
		w.WriteHeader(http.StatusOK)
		data := make([]byte, 2048)
		w.Write(data)
	}))
	defer server.Close()

	options := &DownloadOptions{
		MaxFileSize:    1024, // 1KB limit
		CreateDirs:     true,
		OverwriteExist: true,
		Timeout:        10 * time.Second,
	}

	_, err := DownloadFileUsingHttp("GET", server.URL+"/large-file", nil, nil, nil, options)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file size")
	assert.Contains(t, err.Error(), "exceeds maximum allowed size")
}

func TestDownloadFileUsingHttp_FileNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	options := &DownloadOptions{
		CreateDirs:     true,
		OverwriteExist: true,
		Timeout:        10 * time.Second,
	}

	_, err := DownloadFileUsingHttp("GET", server.URL+"/not-found", nil, nil, nil, options)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file not found")
}

func TestExtractFilenameFromContentDisposition(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "Standard attachment with quotes",
			header:   `attachment; filename="test-file.pdf"`,
			expected: "test-file.pdf",
		},
		{
			name:     "Without quotes",
			header:   `attachment; filename=document.txt`,
			expected: "document.txt",
		},
		{
			name:     "No filename parameter",
			header:   `attachment`,
			expected: "",
		},
		{
			name:     "Empty header",
			header:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFilenameFromContentDisposition(tt.header)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDownloadFileUsingHttp_ResumeDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "bytes=5-" {
			w.Header().Set("Content-Range", "bytes 5-10/11")
			w.Header().Set("Content-Length", "6")
			w.WriteHeader(http.StatusPartialContent)
			w.Write([]byte(" World"))
		} else {
			w.Header().Set("Content-Length", "11")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Hello World"))
		}
	}))
	defer server.Close()

	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "resume-test.txt")

	// Create partial file
	err := os.WriteFile(filePath, []byte("Hello"), 0600)
	require.NoError(t, err)

	options := &DownloadOptions{
		OutputPath:     filePath,
		CreateDirs:     true,
		OverwriteExist: true,
		ResumeDownload: true,
		Timeout:        10 * time.Second,
	}

	result, err := DownloadFileUsingHttp("GET", server.URL+"/resume", nil, nil, nil, options)

	require.NoError(t, err)
	assert.Equal(t, http.StatusPartialContent, result.StatusCode)

	// Verify complete file content
	content, err := os.ReadFile(result.FilePath)
	require.NoError(t, err)
	assert.Equal(t, "Hello World", string(content))
}

func TestDownloadFileUsingHttp_ProgressCallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", "20")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("This is test content"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	var progressCalls []struct {
		downloaded int64
		total      int64
	}

	options := &DownloadOptions{
		OutputPath:     filepath.Join(tempDir, "progress-test.txt"),
		CreateDirs:     true,
		OverwriteExist: true,
		ProgressCallback: func(downloaded, total int64) {
			progressCalls = append(progressCalls, struct {
				downloaded int64
				total      int64
			}{downloaded, total})
		},
		Timeout: 10 * time.Second,
	}

	result, err := DownloadFileUsingHttp("GET", server.URL+"/progress", nil, nil, nil, options)

	require.NoError(t, err)
	assert.Equal(t, int64(20), result.Size)
	assert.True(t, len(progressCalls) > 0, "Progress callback should be called")

	// Verify final progress call
	lastCall := progressCalls[len(progressCalls)-1]
	assert.Equal(t, int64(20), lastCall.downloaded)
	assert.Equal(t, int64(20), lastCall.total)
}

func TestDownloadFileUsingHttp_CustomHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify custom headers were sent
		assert.Equal(t, "test-agent", r.Header.Get("User-Agent"))
		assert.Equal(t, "custom-value", r.Header.Get("X-Custom-Header"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Custom headers received"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	options := &DownloadOptions{
		OutputPath:     filepath.Join(tempDir, "headers-test.txt"),
		CreateDirs:     true,
		OverwriteExist: true,
		Headers: map[string]string{
			"User-Agent":      "test-agent",
			"X-Custom-Header": "custom-value",
			"Accept":          "application/json", // Should override default
		},
		Timeout: 10 * time.Second,
	}

	result, err := DownloadFileUsingHttp("GET", server.URL+"/headers", nil, nil, nil, options)

	require.NoError(t, err)
	content, err := os.ReadFile(result.FilePath)
	require.NoError(t, err)
	assert.Equal(t, "Custom headers received", string(content))
}

func TestDownloadFileUsingHttp_UnsupportedHTTPVerb(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	options := &DownloadOptions{
		CreateDirs:     true,
		OverwriteExist: true,
		Timeout:        10 * time.Second,
	}

	_, err := DownloadFileUsingHttp("INVALID", server.URL+"/test", nil, nil, nil, options)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported HTTP verb: INVALID")
}

func TestDownloadFileUsingHttp_FileExistsNoOverwrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("New content"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	existingFile := filepath.Join(tempDir, "existing.txt")

	// Create existing file
	err := os.WriteFile(existingFile, []byte("Existing content"), 0600)
	require.NoError(t, err)

	options := &DownloadOptions{
		OutputPath:     existingFile,
		CreateDirs:     true,
		OverwriteExist: false, // Don't overwrite
		ResumeDownload: false,
		Timeout:        10 * time.Second,
	}

	_, err = DownloadFileUsingHttp("GET", server.URL+"/test", nil, nil, nil, options)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file already exists")

	// Verify original content is preserved
	content, err := os.ReadFile(filepath.Clean(existingFile))
	require.NoError(t, err)
	assert.Equal(t, "Existing content", string(content))
}
