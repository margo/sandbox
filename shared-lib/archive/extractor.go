package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
)

// BundleExtractor handles extraction of tar.gz bundles
type BundleExtractor struct {
	bundleData []byte
	entries    map[string][]byte
}

// NewExtractor creates a new bundle extractor
func NewExtractor(bundleData []byte) *BundleExtractor {
	return &BundleExtractor{
		bundleData: bundleData,
		entries:    make(map[string][]byte),
	}
}

// Extract extracts all files from the tar.gz bundle
func (e *BundleExtractor) Extract() (map[string][]byte, error) {
	// Create gzip reader
	gzipReader, err := gzip.NewReader(bytes.NewReader(e.bundleData))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() {
		_ = gzipReader.Close()
	}()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	// Extract each file
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Only process regular files
		if header.Typeflag != tar.TypeReg {
			continue
		}

		// Read file content
		content, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", header.Name, err)
		}

		// Store with filename as key
		e.entries[header.Name] = content
	}

	return e.entries, nil
}

// ExtractWithDigestVerification extracts and verifies each file's digest
func (e *BundleExtractor) ExtractWithDigestVerification(expectedDigests map[string]string) (map[string][]byte, error) {
	entries, err := e.Extract()
	if err != nil {
		return nil, err
	}

	// Verify digests if provided
	if expectedDigests != nil {
		for filename, content := range entries {
			expectedDigest, exists := expectedDigests[filename]
			if !exists {
				continue // Skip files without expected digest
			}

			// Compute actual digest
			hash := sha256.Sum256(content)
			actualDigest := fmt.Sprintf("sha256:%x", hash)

			if actualDigest != expectedDigest {
				return nil, fmt.Errorf("digest mismatch for %s: expected %s, got %s",
					filename, expectedDigest, actualDigest)
			}
		}
	}

	return entries, nil
}

// GetEntry retrieves a specific file from the extracted bundle
func (e *BundleExtractor) GetEntry(filename string) ([]byte, error) {
	if e.entries == nil {
		if _, err := e.Extract(); err != nil {
			return nil, err
		}
	}

	content, exists := e.entries[filename]
	if !exists {
		return nil, fmt.Errorf("file not found in bundle: %s", filename)
	}

	return content, nil
}

// ListEntries returns all filenames in the bundle
func (e *BundleExtractor) ListEntries() ([]string, error) {
	if e.entries == nil {
		if _, err := e.Extract(); err != nil {
			return nil, err
		}
	}

	filenames := make([]string, 0, len(e.entries))
	for filename := range e.entries {
		filenames = append(filenames, filename)
	}

	return filenames, nil
}

// VerifyBundleDigest verifies the digest of the entire bundle
func (e *BundleExtractor) VerifyBundleDigest(expectedDigest string) error {
	hash := sha256.Sum256(e.bundleData)
	actualDigest := fmt.Sprintf("sha256:%x", hash)

	if actualDigest != expectedDigest {
		return fmt.Errorf("bundle digest mismatch: expected %s, got %s",
			expectedDigest, actualDigest)
	}

	return nil
}

// GetBundleSize returns the size of the bundle in bytes
func (e *BundleExtractor) GetBundleSize() uint64 {
	return uint64(len(e.bundleData))
}
