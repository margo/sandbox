package archive

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewArchiver(t *testing.T) {
	tests := []struct {
		name   string
		format ArchiveFormats
	}{
		{
			name:   "tar.gz format",
			format: ArchiveFormatTarGZ,
		},
		{
			name:   "custom format",
			format: "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archiver := NewArchiver(tt.format)
			if archiver == nil {
				t.Fatal("NewArchiver returned nil")
			}
			if archiver.ArchiveFormat != tt.format {
				t.Errorf("Expected format %s, got %s", tt.format, archiver.ArchiveFormat)
			}
			if len(archiver.entries) != 0 {
				t.Errorf("Expected empty entries, got %d entries", len(archiver.entries))
			}
		})
	}
}

func TestArchiver_AppendFile(t *testing.T) {
	// Create temporary test file
	tempFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempFile.Name())

	testContent := "test file content"
	if _, err := tempFile.WriteString(testContent); err != nil {
		t.Fatal(err)
	}
	tempFile.Close()

	tests := []struct {
		name                  string
		filePathInsideArchive string
		pathToFile            string
		expectError           bool
		expectedErrorContains string
	}{
		{
			name:                  "valid file",
			filePathInsideArchive: "test/file.txt",
			pathToFile:            tempFile.Name(),
			expectError:           false,
		},
		{
			name:                  "file with leading slash",
			filePathInsideArchive: "/test/file.txt",
			pathToFile:            tempFile.Name(),
			expectError:           false,
		},
		{
			name:                  "empty archive path",
			filePathInsideArchive: "",
			pathToFile:            tempFile.Name(),
			expectError:           true,
			expectedErrorContains: "filePathInsideArchive cannot be empty",
		},
		{
			name:                  "empty file path",
			filePathInsideArchive: "test.txt",
			pathToFile:            "",
			expectError:           true,
			expectedErrorContains: "pathToTheFileYouWantToCopyInArchive cannot be empty",
		},
		{
			name:                  "non-existent file",
			filePathInsideArchive: "test.txt",
			pathToFile:            "/non/existent/file.txt",
			expectError:           true,
			expectedErrorContains: "source file does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archiver := NewArchiver(ArchiveFormatTarGZ)
			err := archiver.AppendFile(tt.filePathInsideArchive, tt.pathToFile)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.expectedErrorContains) {
					t.Errorf(
						"Expected error containing '%s', got '%s'",
						tt.expectedErrorContains,
						err.Error(),
					)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(archiver.entries) != 1 {
					t.Errorf("Expected 1 entry, got %d", len(archiver.entries))
				}
				entry := archiver.entries[0]
				if !entry.IsFile {
					t.Error("Expected entry to be a file")
				}
				expectedPath := strings.TrimPrefix(filepath.Clean(tt.filePathInsideArchive), "/")
				if entry.Name != expectedPath {
					t.Errorf("Expected name %s, got %s", expectedPath, entry.Name)
				}
			}
		})
	}
}

func TestArchiver_AppendContent(t *testing.T) {
	tests := []struct {
		name                  string
		content               []byte
		filePathInArchive     string
		expectError           bool
		expectedErrorContains string
	}{
		{
			name:              "valid content",
			content:           []byte("test content"),
			filePathInArchive: "test/content.txt",
			expectError:       false,
		},
		{
			name:              "empty content",
			content:           []byte{},
			filePathInArchive: "empty.txt",
			expectError:       false,
		},
		{
			name:              "path with leading slash",
			content:           []byte("content"),
			filePathInArchive: "/test/content.txt",
			expectError:       false,
		},
		{
			name:                  "empty file path",
			content:               []byte("content"),
			filePathInArchive:     "",
			expectError:           true,
			expectedErrorContains: "filePathInArchive cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archiver := NewArchiver(ArchiveFormatTarGZ)
			_, _, err := archiver.AppendContent(tt.content, tt.filePathInArchive)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.expectedErrorContains) {
					t.Errorf(
						"Expected error containing '%s', got '%s'",
						tt.expectedErrorContains,
						err.Error(),
					)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(archiver.entries) != 1 {
					t.Errorf("Expected 1 entry, got %d", len(archiver.entries))
				}
				entry := archiver.entries[0]
				if entry.IsFile {
					t.Error("Expected entry to be content, not file")
				}
				expectedPath := strings.TrimPrefix(filepath.Clean(tt.filePathInArchive), "/")
				if entry.Name != expectedPath {
					t.Errorf("Expected name %s, got %s", expectedPath, entry.Name)
				}
				if string(entry.Content) != string(tt.content) {
					t.Errorf(
						"Expected content %s, got %s",
						string(tt.content),
						string(entry.Content),
					)
				}
			}
		})
	}
}

func TestArchiver_CreateArchive(t *testing.T) {
	// Create temporary test file
	tempFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempFile.Name())

	testFileContent := "test file content"
	if _, err := tempFile.WriteString(testFileContent); err != nil {
		t.Fatal(err)
	}
	tempFile.Close()

	tests := []struct {
		name        string
		setupFunc   func(*Archiver) error
		expectError bool
		format      ArchiveFormats
	}{
		{
			name: "successful archive creation with file and content",
			setupFunc: func(a *Archiver) error {
				if err := a.AppendFile("test-file.txt", tempFile.Name()); err != nil {
					return err
				}
				_, _, err := a.AppendContent([]byte("test content"), "test-content.txt")
				return err
			},
			expectError: false,
			format:      ArchiveFormatTarGZ,
		},
		{
			name: "empty archive",
			setupFunc: func(a *Archiver) error {
				return nil
			},
			expectError: false,
			format:      ArchiveFormatTarGZ,
		},
		{
			name: "unsupported format",
			setupFunc: func(a *Archiver) error {
				_, _, err := a.AppendContent([]byte("content"), "file.txt")
				return err
			},
			expectError: true,
			format:      "unsupported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archiver := NewArchiver(tt.format)

			if err := tt.setupFunc(archiver); err != nil {
				t.Fatal(err)
			}

			archiveObj, digest, size, path, err := archiver.CreateArchive()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Cleanup
			defer func() {
				if archiveObj != nil {
					archiveObj.Close()
				}
				archiver.Cleanup()
			}()

			// Validate results
			if archiveObj == nil {
				t.Error("Expected archive object, got nil")
			}

			if digest == "" {
				t.Error("Expected digest, got empty string")
			}

			if !strings.HasPrefix(digest, "sha256:") {
				t.Errorf("Expected digest to start with 'sha256:', got %s", digest)
			}

			if size == 0 {
				t.Error("Expected size > 0, got 0")
			}

			if path == "" {
				t.Error("Expected path, got empty string")
			}

			// Verify file exists
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("Archive file does not exist at path: %s", path)
			}

			// Verify digest calculation
			expectedDigest, expectedSize := calculateExpectedDigest(t, path)
			if digest != expectedDigest {
				t.Errorf("Expected digest %s, got %s", expectedDigest, digest)
			}
			if size != expectedSize {
				t.Errorf("Expected size %d, got %d", expectedSize, size)
			}

			// Verify archive content
			verifyArchiveContent(t, path, archiver.GetEntries())
		})
	}
}

// Helper function to calculate expected digest
func calculateExpectedDigest(t *testing.T, filePath string) (string, uint64) {
	file, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		t.Fatal(err)
	}

	digest := hex.EncodeToString(hasher.Sum(nil))
	size := uint64(0)
	if fileInfo.Size() > 0 {
		// #nosec G115 -- value is checked above
		size = uint64(fileInfo.Size())
	}
	return fmt.Sprintf("sha256:%s", digest), size
}

// Helper function to verify archive content
func verifyArchiveContent(t *testing.T, archivePath string, expectedEntries []ArchiveEntry) {
	file, err := os.Open(filepath.Clean(archivePath))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	foundEntries := make(map[string]bool)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}

		foundEntries[header.Name] = true

		// Find corresponding expected entry
		var expectedEntry *ArchiveEntry
		for _, entry := range expectedEntries {
			if entry.Name == header.Name {
				expectedEntry = &entry
				break
			}
		}

		if expectedEntry == nil {
			t.Errorf("Unexpected entry in archive: %s", header.Name)
			continue
		}

		// Read content and verify
		content, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatal(err)
		}

		if expectedEntry.IsFile {
			// Read expected content from file
			expectedContent, err := os.ReadFile(expectedEntry.Path)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != string(expectedContent) {
				t.Errorf("Content mismatch for %s", header.Name)
			}
		} else {
			// Compare with expected content
			if string(content) != string(expectedEntry.Content) {
				t.Errorf("Content mismatch for %s: expected %s, got %s",
					header.Name, string(expectedEntry.Content), string(content))
			}
		}
	}

	// Verify all expected entries were found
	for _, entry := range expectedEntries {
		if !foundEntries[entry.Name] {
			t.Errorf("Expected entry not found in archive: %s", entry.Name)
		}
	}
}
