package cache

// BundleCache provides bundle-specific caching operations
type BundleCache struct {
	cache *Cache
}

// NewBundleCache creates a new bundle cache
func NewBundleCache(baseDir string) (*BundleCache, error) {
	cache, err := NewCache(baseDir)
	if err != nil {
		return nil, err
	}

	return &BundleCache{cache: cache}, nil
}

// StoreBundle stores a bundle with digest verification
func (bc *BundleCache) StoreBundle(deviceId, digest string, data []byte) error {
	return bc.cache.Store(CacheTypeBundle, deviceId, digest, data)
}

// GetBundle retrieves a cached bundle
func (bc *BundleCache) GetBundle(deviceId, digest string) ([]byte, error) {
	return bc.cache.Get(CacheTypeBundle, deviceId, digest)
}

// GetLastBundleDigest retrieves the last cached bundle digest for a device
func (bc *BundleCache) GetLastBundleDigest(deviceId string) (string, error) {
	return bc.cache.GetLastDigest(CacheTypeBundle, deviceId)
}

// BundleExists checks if a bundle is cached
func (bc *BundleCache) BundleExists(deviceId, digest string) bool {
	return bc.cache.Exists(CacheTypeBundle, deviceId, digest)
}

// DeleteBundle removes a cached bundle
func (bc *BundleCache) DeleteBundle(deviceId, digest string) error {
	return bc.cache.Delete(CacheTypeBundle, deviceId, digest)
}

// ClearDeviceBundles removes all bundles for a device
func (bc *BundleCache) ClearDeviceBundles(deviceId string) error {
	return bc.cache.Clear(CacheTypeBundle, deviceId)
}

// GetBundleCacheStats returns bundle cache statistics
func (bc *BundleCache) GetBundleCacheStats() (totalSize int64, fileCount int, err error) {
	return bc.cache.GetCacheStats(CacheTypeBundle)
}
