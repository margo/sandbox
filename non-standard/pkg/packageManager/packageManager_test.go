package packageManager

// import (
// 	"os"
// 	"testing"
// 	"time"

// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"
// )

// // TestLoadPackageFromOci_Success tests successful package loading from OCI registry
// // Note: This test requires a real OCI registry or mock implementation
// // TODO: Introduce mock OCI registry for isolated testing
// func TestLoadPackageFromOci_Success(t *testing.T) {
// 	t.Skip("Skipping integration test - requires OCI registry setup or mocks")

// 	pm := NewPackageManager()
// 	pkgPath, pkg, err := pm.LoadPackageFromOci(
// 		"docker.io",
// 		"testuser/testapp",
// 		"v1.0.0",
// 		"testuser",
// 		"testtoken",
// 		false,
// 		time.Second*30,
// 	)

// 	require.NoError(t, err)
// 	require.NotNil(t, pkg)
// 	require.NotEmpty(t, pkgPath)
// 	defer os.RemoveAll(pkgPath) // Cleanup temporary directory

// 	// Verify package path exists
// 	_, err = os.Stat(pkgPath)
// 	assert.NoError(t, err, "package path should exist")

// 	// Verify package content
// 	assert.NotNil(t, pkg.Description)
// 	assert.NotEmpty(t, pkg.Description.Metadata.Name)
// 	assert.NotEmpty(t, pkg.Description.Metadata.Version)
// }

// // TestLoadPackageFromOci_InvalidRegistry tests error handling for invalid registry
// func TestLoadPackageFromOci_InvalidRegistry(t *testing.T) {
// 	pm := NewPackageManager()
// 	pkgPath, pkg, err := pm.LoadPackageFromOci(
// 		"",
// 		"testuser/testapp",
// 		"v1.0.0",
// 		"",
// 		"",
// 		false,
// 		time.Second*30,
// 	)

// 	require.Error(t, err)
// 	assert.Contains(t, err.Error(), "failed to initialize OCI client")
// 	assert.Empty(t, pkgPath)
// 	assert.Nil(t, pkg)
// }

// // TestLoadPackageFromOci_CleanupOnFailure tests that temp directories are cleaned up on failure
// // Note: Mock OCI client can be introduced here to simulate pull failures
// // TODO: Add mock to test cleanup behavior without external dependencies
// func TestLoadPackageFromOci_CleanupOnFailure(t *testing.T) {
// 	t.Skip("Skipping - requires mock OCI client to simulate failures")

// 	pm := NewPackageManager()
// 	pkgPath, pkg, err := pm.LoadPackageFromOci(
// 		"docker.io",
// 		"nonexistent/repo",
// 		"nonexistent",
// 		"",
// 		"",
// 		false,
// 		time.Second*30,
// 	)

// 	require.Error(t, err)
// 	assert.Empty(t, pkgPath)
// 	assert.Nil(t, pkg)

// 	// Verify no temporary directories are left behind
// 	// This would be properly tested with mocks
// }
