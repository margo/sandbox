package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// CacheType represents different types of cached resources
type CacheType string

const (
	CacheTypeBundle     CacheType = "bundles"
	CacheTypeDeployment CacheType = "deployments"
)

// Cache provides a generic caching layer for content-addressable resources
type Cache struct {
	baseDir string
	mu      sync.RWMutex
}

// NewCache creates a new cache instance
func NewCache(baseDir string) (*Cache, error) {
	if err := os.MkdirAll(baseDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Cache{
		baseDir: baseDir,
	}, nil
}

// Store stores data with digest verification
func (c *Cache) Store(cacheType CacheType, key, digest string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Verify digest before storing (Exact Bytes Rule)
	hash := sha256.Sum256(data)
	actualDigest := fmt.Sprintf("sha256:%x", hash)
	if actualDigest != digest {
		return fmt.Errorf("digest mismatch: expected %s, got %s", digest, actualDigest)
	}

	// Create cache path
	cachePath := filepath.Join(c.baseDir, string(cacheType), key, digest)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0750); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Write data
	if err := os.WriteFile(cachePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	// Update metadata
	return c.updateMetadata(cacheType, key, digest)
}

// Get retrieves cached data with integrity verification
func (c *Cache) Get(cacheType CacheType, key, digest string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cachePath := filepath.Join(c.baseDir, string(cacheType), key, digest)
	data, err := os.ReadFile(filepath.Clean(cachePath))
	if err != nil {
		return nil, fmt.Errorf("cache miss: %w", err)
	}

	// Verify integrity (Exact Bytes Rule)
	hash := sha256.Sum256(data)
	actualDigest := fmt.Sprintf("sha256:%x", hash)
	if actualDigest != digest {
		// Cache corruption detected - remove corrupted file
		_ = os.Remove(cachePath)
		return nil, fmt.Errorf(
			"cache corruption detected: expected %s, got %s",
			digest,
			actualDigest,
		)
	}

	return data, nil
}

// GetLastDigest retrieves the last cached digest for a key
func (c *Cache) GetLastDigest(cacheType CacheType, key string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metaPath := filepath.Join(c.baseDir, string(cacheType), key, "metadata.json")
	data, err := os.ReadFile(filepath.Clean(metaPath))
	if err != nil {
		return "", fmt.Errorf("no cached metadata: %w", err)
	}

	var meta struct {
		LastDigest string `json:"lastDigest"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return "", fmt.Errorf("failed to parse metadata: %w", err)
	}

	return meta.LastDigest, nil
}

// Exists checks if a specific digest is cached
func (c *Cache) Exists(cacheType CacheType, key, digest string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cachePath := filepath.Join(c.baseDir, string(cacheType), key, digest)
	_, err := os.Stat(cachePath)
	return err == nil
}

// Delete removes a cached entry
func (c *Cache) Delete(cacheType CacheType, key, digest string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cachePath := filepath.Join(c.baseDir, string(cacheType), key, digest)
	return os.Remove(cachePath)
}

// Clear removes all cached entries for a specific key
func (c *Cache) Clear(cacheType CacheType, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	keyPath := filepath.Join(c.baseDir, string(cacheType), key)
	return os.RemoveAll(keyPath)
}

// ClearAll removes all cached entries of a specific type
func (c *Cache) ClearAll(cacheType CacheType) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	typePath := filepath.Join(c.baseDir, string(cacheType))
	return os.RemoveAll(typePath)
}

// updateMetadata updates the metadata file with the latest digest
func (c *Cache) updateMetadata(cacheType CacheType, key, digest string) error {
	metaPath := filepath.Join(c.baseDir, string(cacheType), key, "metadata.json")

	meta := struct {
		LastDigest string `json:"lastDigest"`
		UpdatedAt  string `json:"updatedAt"`
	}{
		LastDigest: digest,
		UpdatedAt:  fmt.Sprintf("%d", os.Getpid()), // Simple timestamp alternative
	}

	metaData, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	return os.WriteFile(metaPath, metaData, 0600)
}

// GetCacheStats returns statistics about the cache
func (c *Cache) GetCacheStats(cacheType CacheType) (totalSize int64, fileCount int, err error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	typePath := filepath.Join(c.baseDir, string(cacheType))

	err = filepath.Walk(typePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) != ".json" {
			totalSize += info.Size()
			fileCount++
		}
		return nil
	})

	return totalSize, fileCount, err
}
