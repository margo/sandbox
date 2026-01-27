package cache

// DeploymentCache provides deployment-specific caching operations
type DeploymentCache struct {
	cache *Cache
}

// NewDeploymentCache creates a new deployment cache
func NewDeploymentCache(baseDir string) (*DeploymentCache, error) {
	cache, err := NewCache(baseDir)
	if err != nil {
		return nil, err
	}

	return &DeploymentCache{cache: cache}, nil
}

// StoreDeployment stores a deployment YAML with digest verification
func (dc *DeploymentCache) StoreDeployment(deploymentId, digest string, data []byte) error {
	return dc.cache.Store(CacheTypeDeployment, deploymentId, digest, data)
}

// GetDeployment retrieves a cached deployment YAML
func (dc *DeploymentCache) GetDeployment(deploymentId, digest string) ([]byte, error) {
	return dc.cache.Get(CacheTypeDeployment, deploymentId, digest)
}

// GetLastDeploymentDigest retrieves the last cached deployment digest
func (dc *DeploymentCache) GetLastDeploymentDigest(deploymentId string) (string, error) {
	return dc.cache.GetLastDigest(CacheTypeDeployment, deploymentId)
}

// DeploymentExists checks if a deployment is cached
func (dc *DeploymentCache) DeploymentExists(deploymentId, digest string) bool {
	return dc.cache.Exists(CacheTypeDeployment, deploymentId, digest)
}

// DeleteDeployment removes a cached deployment
func (dc *DeploymentCache) DeleteDeployment(deploymentId, digest string) error {
	return dc.cache.Delete(CacheTypeDeployment, deploymentId, digest)
}

// ClearDeploymentCache removes all cached versions of a deployment
func (dc *DeploymentCache) ClearDeploymentCache(deploymentId string) error {
	return dc.cache.Clear(CacheTypeDeployment, deploymentId)
}

// GetDeploymentCacheStats returns deployment cache statistics
func (dc *DeploymentCache) GetDeploymentCacheStats() (totalSize int64, fileCount int, err error) {
	return dc.cache.GetCacheStats(CacheTypeDeployment)
}
