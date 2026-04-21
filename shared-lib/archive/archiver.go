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
	"sort"
	"strings"
	"time"
)

type ArchiveFormats string

const (
	ArchiveFormatTarGZ ArchiveFormats = "tar.gz"
)

type ArchiveEntry struct {
	Name    string
	Content []byte
	Path    string // For file-based entries
	IsFile  bool   // true for files, false for content
}

type Archiver struct {
	ArchiveFormat ArchiveFormats
	entries       []ArchiveEntry
	outputPath    string
}

func NewArchiver(format ArchiveFormats) *Archiver {
	return &Archiver{
		ArchiveFormat: format,
		entries:       make([]ArchiveEntry, 0),
	}
}

// AppendFile adds a file from the filesystem to the archive
func (a *Archiver) AppendFile(
	filePathInsideArchive string,
	pathToTheFileYouWantToCopyInArchive string,
) error {
	// Validate input
	if filePathInsideArchive == "" {
		return fmt.Errorf("filePathInsideArchive cannot be empty")
	}
	if pathToTheFileYouWantToCopyInArchive == "" {
		return fmt.Errorf("pathToTheFileYouWantToCopyInArchive cannot be empty")
	}

	// Check if source file exists
	if _, err := os.Stat(pathToTheFileYouWantToCopyInArchive); os.IsNotExist(err) {
		return fmt.Errorf("source file does not exist: %s", pathToTheFileYouWantToCopyInArchive)
	}

	// Clean the archive path
	cleanPath := strings.TrimPrefix(filepath.Clean(filePathInsideArchive), "/")

	entry := ArchiveEntry{
		Name:   cleanPath,
		Path:   pathToTheFileYouWantToCopyInArchive,
		IsFile: true,
	}

	a.entries = append(a.entries, entry)
	return nil
}

// AppendContent adds content directly to the archive
func (a *Archiver) AppendContent(
	content []byte,
	filePathInArchive string,
) (digest string, size uint64, err error) {
	// Validate input
	if filePathInArchive == "" {
		return "", 0, fmt.Errorf("filePathInArchive cannot be empty")
	}

	// Clean the archive path
	cleanPath := strings.TrimPrefix(filepath.Clean(filePathInArchive), "/")

	entry := ArchiveEntry{
		Name:    cleanPath,
		Content: content,
		IsFile:  false,
	}

	a.entries = append(a.entries, entry)
	return "", 0, nil
}

// CreateArchive creates the archive and returns archive info
func (a *Archiver) CreateArchive() (archiveObject *os.File, digest string, size uint64, pathToArchive string, err error) {
	// Generate temporary file for archive
	tempFile, err := os.CreateTemp("", "archive-*.tar.gz")
	if err != nil {
		return nil, "", 0, "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	a.outputPath = tempFile.Name()

	// Create archive based on format
	switch a.ArchiveFormat {
	case ArchiveFormatTarGZ:
		err = a.createTarGzArchive(tempFile)
	default:
		return nil, "", 0, "", fmt.Errorf("unsupported archive format: %s", a.ArchiveFormat)
	}

	if err != nil {
		_ = os.Remove(a.outputPath)
		return nil, "", 0, "", err
	}

	// Calculate digest and size
	digest, size, err = a.calculateDigestAndSize()
	if err != nil {
		_ = os.Remove(a.outputPath)
		return nil, "", 0, "", err
	}

	// Return archive file as archiveObject
	archiveFile, err := os.Open(a.outputPath)
	if err != nil {
		_ = os.Remove(a.outputPath)
		return nil, "", 0, "", fmt.Errorf("failed to open created archive: %w", err)
	}

	return archiveFile, digest, size, a.outputPath, nil
}

// createTarGzArchive creates a tar.gz archive
func (a *Archiver) createTarGzArchive(output *os.File) (err error) {
	//  Sort entries for deterministic ordering
	a.sortEntries()

	// Create gzip writer with deterministic settings
	gzipWriter := gzip.NewWriter(output)
	gzipWriter.Name = ""                                             //  Clear filename for reproducibility
	gzipWriter.Comment = ""                                          //  Clear comment for reproducibility
	gzipWriter.ModTime = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC) //  Fixed timestamp
	defer func() {
		closeErr := gzipWriter.Close()
		if err == nil {
			err = closeErr
		}
	}()

	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer func() {
		closeErr := tarWriter.Close()
		if err == nil {
			err = closeErr
		}
	}()

	// Add all entries to archive (now sorted)
	for _, entry := range a.entries {
		if entry.IsFile {
			err := a.addFileToTar(tarWriter, entry.Name, entry.Path)
			if err != nil {
				return fmt.Errorf("failed to add file %s: %w", entry.Path, err)
			}
		} else {
			err := a.addContentToTar(tarWriter, entry.Name, entry.Content)
			if err != nil {
				return fmt.Errorf("failed to add content %s: %w", entry.Name, err)
			}
		}
	}

	return nil
}

// addFileToTar adds a file from filesystem to tar archive
func (a *Archiver) addFileToTar(tarWriter *tar.Writer, nameInArchive, filePath string) error {
	file, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	// Use fixed timestamp for reproducible archives
	fixedTime := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create tar header
	header := &tar.Header{
		Name:    nameInArchive,
		Size:    fileInfo.Size(),
		Mode:    int64(fileInfo.Mode()),
		ModTime: fixedTime,
	}

	// Write header
	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	// Copy file content
	_, err = io.Copy(tarWriter, file)
	return err
}

// Update addContentToTar to use fixed timestamps
func (a *Archiver) addContentToTar(
	tarWriter *tar.Writer,
	nameInArchive string,
	content []byte,
) error {
	// Use fixed timestamp for reproducible archives (Exact Bytes Rule)
	fixedTime := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create tar header
	header := &tar.Header{
		Name:    nameInArchive,
		Size:    int64(len(content)),
		Mode:    0644,
		ModTime: fixedTime, // ✅ Fixed timestamp
	}

	// Write header
	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	// Write content
	_, err := tarWriter.Write(content)
	return err
}

// calculateDigestAndSize calculates SHA256 digest and file size
func (a *Archiver) calculateDigestAndSize() (string, uint64, error) {
	file, err := os.Open(a.outputPath)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return "", 0, err
	}

	// #nosec G115 -- check has been added to ensure no overflow
	size := uint64(fileInfo.Size())
	if fileInfo.Size() <= 0 {
		size = 0
	}

	// Calculate SHA256 hash
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", 0, err
	}

	digest := hex.EncodeToString(hasher.Sum(nil))
	return fmt.Sprintf("sha256:%s", digest), size, nil
}

// GetEntries returns the list of entries that will be/were added to archive
func (a *Archiver) GetEntries() []ArchiveEntry {
	return a.entries
}

// Clear removes all entries from the archiver
func (a *Archiver) Clear() {
	a.entries = make([]ArchiveEntry, 0)
}

// SetOutputPath sets a custom output path for the archive
func (a *Archiver) SetOutputPath(path string) {
	a.outputPath = path
}

// Cleanup removes the created archive file
func (a *Archiver) Cleanup() error {
	if a.outputPath != "" {
		return os.Remove(a.outputPath)
	}
	return nil
}
func (a *Archiver) sortEntries() {
	sort.Slice(a.entries, func(i, j int) bool {
		return a.entries[i].Name < a.entries[j].Name
	})
}
