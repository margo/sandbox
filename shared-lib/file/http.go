package file

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	httputils "github.com/margo/dev-repo/shared-lib/http"
	"github.com/margo/dev-repo/shared-lib/http/auth"
)

// DownloadResult contains information about the download operation
type DownloadResult struct {
	FilePath     string
	Size         int64
	ContentType  string
	LastModified time.Time
	ETag         string
	StatusCode   int
}

// DownloadOptions provides configuration for file downloads
type DownloadOptions struct {
	OutputPath       string                        // Where to save the file
	CreateDirs       bool                          // Create directories if they don't exist
	OverwriteExist   bool                          // Overwrite existing files
	MaxFileSize      int64                         // Maximum file size to download (0 = no limit)
	Timeout          time.Duration                 // HTTP request timeout
	Headers          map[string]string             // Additional headers
	ResumeDownload   bool                          // Resume partial downloads
	ProgressCallback func(downloaded, total int64) // Progress callback
}

// DownloadFileUsingHttp downloads a file using the specified HTTP method with authentication
func DownloadFileUsingHttp(httpVerb, url string, auth *auth.AuthConfig, queryParams map[string]interface{}, body interface{}, options *DownloadOptions) (*DownloadResult, error) {
	// Set default options if not provided
	if options == nil {
		options = &DownloadOptions{
			CreateDirs:     true,
			OverwriteExist: true,
			MaxFileSize:    100 * 1024 * 1024, // 100MB default limit
			Timeout:        30 * time.Second,
		}
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: options.Timeout,
	}

	// Create HTTP request using the reusable methods
	req, err := createHTTPRequest(httpVerb, url, auth, queryParams, body, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// // Handle resume download if requested
	if options.ResumeDownload && options.OutputPath != "" {
		if err := addResumeHeaders(req, options.OutputPath); err != nil {
			return nil, fmt.Errorf("failed to add resume headers: %w", err)
		}
	}

	// Add download-specific headers
	setDownloadHeaders(req)

	// Add custom headers (these will override defaults if same key)
	if options.Headers != nil {
		for key, value := range options.Headers {
			req.Header.Set(key, value)
		}
	}

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if err := validateResponse(resp, options.ResumeDownload); err != nil {
		return nil, err
	}

	// Determine output path
	// filename, err := generateFilename(url, resp)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to generate output filename: %w", err)
	// }
	// outputPath := filepath.Join(options.OutputPath, filename)
	outputPath := options.OutputPath

	// Create directories if needed
	if options.CreateDirs {
		dir := filepath.Dir(outputPath)
		if dir != "." && dir != "/" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directories: %w", err)
			}
		}
	}

	// Check if file exists and handle overwrite
	if !options.OverwriteExist && !options.ResumeDownload {
		if _, err := os.Stat(outputPath); err == nil {
			return nil, fmt.Errorf("file already exists: %s", outputPath)
		}
	}

	// Download the file
	result, err := downloadFile(resp, outputPath, options)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}

	return result, nil
}

// createHTTPRequest creates an HTTP request using the reusable HTTP utility methods
func createHTTPRequest(httpVerb, url string, auth *auth.AuthConfig, queryParams map[string]interface{}, body interface{}, options *DownloadOptions) (*http.Request, error) {
	// Normalize HTTP verb
	httpVerb = strings.ToUpper(httpVerb)

	var req *http.Request
	var err error

	// Use the appropriate HTTP utility method based on verb
	switch httpVerb {
	case "GET":
		req, err = httputils.NewGetRequest(url, auth, queryParams)
	case "POST":
		contentType := getContentTypeFromHeaders(options.Headers)
		req, err = httputils.NewPostRequest(url, auth, body, contentType)
	case "PUT":
		contentType := getContentTypeFromHeaders(options.Headers)
		req, err = httputils.NewPutRequest(url, auth, body, contentType)
	case "PATCH":
		contentType := getContentTypeFromHeaders(options.Headers)
		req, err = httputils.NewPatchRequest(url, auth, body, contentType)
	case "DELETE":
		req, err = httputils.NewDeleteRequest(url, auth, queryParams)
	case "HEAD":
		req, err = httputils.NewHeadRequest(url, auth, queryParams)
	case "OPTIONS":
		req, err = httputils.NewOptionsRequest(url, auth)
	default:
		return nil, fmt.Errorf("unsupported HTTP verb: %s", httpVerb)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create %s request: %w", httpVerb, err)
	}

	return req, nil
}

// getContentTypeFromHeaders extracts content-type from headers map
func getContentTypeFromHeaders(headers map[string]string) string {
	if headers == nil {
		return ""
	}

	// Check for various case variations of Content-Type
	for key, value := range headers {
		if strings.ToLower(key) == "content-type" {
			return value
		}
	}

	return ""
}

// addResumeHeaders adds Range header for resuming downloads
func addResumeHeaders(req *http.Request, outputPath string) error {
	if stat, err := os.Stat(outputPath); err == nil {
		// File exists, add Range header to resume from current size
		if stat.Size() > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", stat.Size()))
		}
	}
	return nil
}

// validateResponse validates the HTTP response
func validateResponse(resp *http.Response, resumeDownload bool) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusPartialContent:
		if resumeDownload {
			return nil
		}
		return fmt.Errorf("unexpected partial content response")
	case http.StatusUnauthorized:
		return fmt.Errorf("authentication failed: HTTP 401")
	case http.StatusForbidden:
		return fmt.Errorf("access forbidden: HTTP 403")
	case http.StatusNotFound:
		return fmt.Errorf("file not found: HTTP 404")
	case http.StatusRequestedRangeNotSatisfiable:
		if resumeDownload {
			// File might already be complete, treat as success
			return nil
		}
		return fmt.Errorf("range not satisfiable: file may be complete")
	default:
		return fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}
}

// generateFilename generates an output path from URL and response headers
func generateFilename(url string, resp *http.Response) (string, error) {
	// Try to get filename from Content-Disposition header
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if filename := extractFilenameFromContentDisposition(cd); filename != "" {
			return filename, nil
		}
	}

	// Extract filename from URL
	if filename := filepath.Base(url); filename != "" && filename != "." && filename != "/" {
		// Remove query parameters
		if idx := strings.Index(filename, "?"); idx != -1 {
			filename = filename[:idx]
		}
		return filename, nil
	}

	// Generate a default filename
	return fmt.Sprintf("download_%d", time.Now().Unix()), nil
}

// downloadFile performs the actual file download
func downloadFile(resp *http.Response, outputPath string, options *DownloadOptions) (*DownloadResult, error) {
	// Get content length
	contentLength := resp.ContentLength
	if contentLengthStr := resp.Header.Get("Content-Length"); contentLengthStr != "" {
		if cl, err := strconv.ParseInt(contentLengthStr, 10, 64); err == nil {
			contentLength = cl
		}
	}

	// Check file size limit
	if options.MaxFileSize > 0 && contentLength > options.MaxFileSize {
		return nil, fmt.Errorf("file size (%d bytes) exceeds maximum allowed size (%d bytes)", contentLength, options.MaxFileSize)
	}

	// Open output file
	var file *os.File
	var err error

	if options.ResumeDownload && resp.StatusCode == http.StatusPartialContent {
		// Open file for appending
		file, err = os.OpenFile(outputPath, os.O_WRONLY|os.O_APPEND, 0644)
	} else {
		// Create new file or truncate existing
		file, err = os.Create(outputPath)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Get initial file size for resume
	var initialSize int64
	if stat, err := file.Stat(); err == nil {
		initialSize = stat.Size()
	}

	// Create progress reader if callback is provided
	var reader io.Reader = resp.Body
	if options.ProgressCallback != nil {
		reader = &progressReader{
			reader:   resp.Body,
			total:    contentLength,
			current:  initialSize,
			callback: options.ProgressCallback,
		}
	}

	// Copy data with size limit
	var written int64
	if options.MaxFileSize > 0 {
		limitedReader := io.LimitReader(reader, options.MaxFileSize)
		written, err = io.Copy(file, limitedReader)
	} else {
		written, err = io.Copy(file, reader)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Parse Last-Modified header
	var lastModified time.Time
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		if t, err := time.Parse(time.RFC1123, lm); err == nil {
			lastModified = t
		}
	}

	// Create result
	result := &DownloadResult{
		FilePath:     outputPath,
		Size:         written + initialSize,
		ContentType:  resp.Header.Get("Content-Type"),
		LastModified: lastModified,
		ETag:         resp.Header.Get("ETag"),
		StatusCode:   resp.StatusCode,
	}

	return result, nil
}

// progressReader wraps an io.Reader to provide progress callbacks
type progressReader struct {
	reader   io.Reader
	total    int64
	current  int64
	callback func(downloaded, total int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.current += int64(n)
	if pr.callback != nil {
		pr.callback(pr.current, pr.total)
	}
	return n, err
}

// extractFilenameFromContentDisposition extracts filename from Content-Disposition header
func extractFilenameFromContentDisposition(cd string) string {
	// Simple extraction - in production, you might want to use a more robust parser
	parts := strings.Split(cd, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "filename=") {
			filename := strings.TrimPrefix(part, "filename=")
			filename = strings.Trim(filename, `"`)
			return filename
		}
	}
	return ""
}

// setDownloadHeaders sets appropriate headers for file downloads
func setDownloadHeaders(req *http.Request) {
	// Override some default headers for downloads
	// req.Header.Set("Accept", "*/*")
	// req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Accept", "application/json, application/yaml, application/x-yaml, text/yaml, text/plain, */*")
	req.Header.Set("User-Agent", "margo-device-agent/1.0")
	req.Header.Set("Accept-Encoding", "identity") // Request uncompressed content
}
