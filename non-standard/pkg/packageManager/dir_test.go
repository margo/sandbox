package packageManager

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectoryLoader_Load_ValidPackageWithResources(t *testing.T) {
	pkgDir := createValidTestPackage(t, true)
	defer os.RemoveAll(pkgDir)

	loader := NewDirectoryLoader(defaultConfig())
	_, pkg, err := loader.Load(context.Background(), pkgDir, &LoadOptions{
		Validate: true,
	})

	require.NoError(t, err)
	assert.NotNil(t, pkg)
	assert.NotNil(t, pkg.Description)
	assert.Equal(t, "test-app", pkg.Description.Metadata.Name)
	assert.NotEmpty(t, pkg.Resources)
}

func TestDirectoryLoader_Load_ValidPackageWithoutResources(t *testing.T) {
	pkgDir := createValidTestPackage(t, false)
	defer os.RemoveAll(pkgDir)

	loader := NewDirectoryLoader(defaultConfig())
	_, pkg, err := loader.Load(context.Background(), pkgDir, &LoadOptions{
		Validate: true,
	})

	require.NoError(t, err)
	assert.NotNil(t, pkg)
	assert.Equal(t, "test-app", pkg.Description.Metadata.Name)
	assert.Empty(t, pkg.Resources)
}

func TestDirectoryLoader_Load_PackageNotFound(t *testing.T) {
	loader := NewDirectoryLoader(defaultConfig())
	_, pkg, err := loader.Load(
		context.Background(),
		"/non/existent/path",
		nil,
	)

	assert.Error(t, err)
	assert.Nil(t, pkg)
	assert.IsType(t, &ErrPackageNotFound{}, err)
}

func TestDirectoryLoader_Load_MissingDescriptionFile(t *testing.T) {
	dir := t.TempDir()

	loader := NewDirectoryLoader(defaultConfig())
	_, pkg, err := loader.Load(context.Background(), dir, &LoadOptions{
		Validate: true,
	})

	assert.Error(t, err)
	assert.Nil(t, pkg)
	assert.IsType(t, &ErrDescriptionNotFound{}, err)
}

func TestDirectoryLoader_Load_InvalidDescriptionFile(t *testing.T) {
	dir := t.TempDir()
	invalidYAML := `
kind: WrongKind
metadata:
  name: test
`
	err := os.WriteFile(
		filepath.Join(dir, ExpectedDescriptionFileName),
		[]byte(invalidYAML),
		0600,
	)
	require.NoError(t, err)

	loader := NewDirectoryLoader(defaultConfig())
	_, pkg, err := loader.Load(context.Background(), dir, &LoadOptions{
		Validate: true,
	})

	assert.Error(t, err)
	assert.Nil(t, pkg)
	assert.IsType(t, &ErrInvalidDescription{}, err)
}

func TestDirectoryLoader_Load_CorruptedYamlFile(t *testing.T) {
	dir := t.TempDir()
	corruptedYAML := `
kind: ApplicationDescription
metadata:
  name: test
  invalid yaml syntax here: [[[
`
	err := os.WriteFile(
		filepath.Join(dir, ExpectedDescriptionFileName),
		[]byte(corruptedYAML),
		0600,
	)
	require.NoError(t, err)

	loader := NewDirectoryLoader(defaultConfig())
	_, pkg, err := loader.Load(context.Background(), dir, &LoadOptions{
		Validate: true,
	})

	assert.Error(t, err)
	assert.Nil(t, pkg)
}

func TestDirectoryLoader_Type(t *testing.T) {
	loader := NewDirectoryLoader(defaultConfig())
	assert.Equal(t, SourceDirectory, loader.Type())
}

// Helper functions
func createValidTestPackage(t *testing.T, withResources bool) string {
	dir := t.TempDir()

	createValidDescription(t, dir)

	if withResources {
		resourcesDir := filepath.Join(dir, "resources")
		require.NoError(t, os.MkdirAll(resourcesDir, 0750))

		err := os.WriteFile(
			filepath.Join(resourcesDir, "icon.png"),
			[]byte("fake-icon"),
			0600,
		)
		require.NoError(t, err)

		err = os.WriteFile(
			filepath.Join(resourcesDir, "description.md"),
			[]byte("# Test Description"),
			0600,
		)
		require.NoError(t, err)

		err = os.WriteFile(
			filepath.Join(resourcesDir, "release-notes.md"),
			[]byte("# Test Release Notes"),
			0600,
		)
		require.NoError(t, err)

		err = os.WriteFile(
			filepath.Join(resourcesDir, "license.pdf"),
			[]byte(""),
			0600,
		)
		require.NoError(t, err)
	}

	return dir
}

func createValidDescription(t *testing.T, dir string) {
	validYAML := `
apiVersion: margo.org/v1-alpha1
kind: ApplicationDescription
metadata:
  id: some-unique-id
  name: test-app
  description: A basic test-app application
  version: "1.0"
  catalog:
    application:
      tagline: test-app application.
      icon: ./resources/icon.png
      descriptionFile: ./resources/description.md
      releaseNotes: ./resources/release-notes.md
      licenseFile: ./resources/license.pdf
      site: http://www.somevalidewebsiteurlhere.com
      tags: ["monitoring"]
    author:
      - name: Great Star
        email: greatstar@somevalidwebsiteurlhere.com
    organization:
      - name: Some Corportation Name
        site: http://somevalidwebsiteurlhere.com
deploymentProfiles:
  - type: helm.v3
    id: test-helm-profile
    components:
      - name: hello-world
        properties:
          repository: oci://somehelmregistryurl.com/charts/hello-world
          revision: 1.0.0
          wait: true
parameters:
  greeting:
    value: Hello
    targets:
    - pointer: global.config.appGreeting
      components: ["hello-world"]
  greetingAddressee:
    value: World
    targets:
    - pointer: global.config.appGreetingAddressee
      components: ["hello-world"]
configuration:
  sections:
    - name: General Settings
      settings:
        - parameter: greeting
          name: Greeting
          description: The greeting to use.
          schema: requireText
        - parameter: greetingAddressee
          name: Greeting Addressee
          description: The person, or group, the greeting addresses.
          schema: requireText
  schema:
    - name: requireText
      dataType: string
      maxLength: 45
      allowEmpty: false
`
	err := os.WriteFile(
		filepath.Join(dir, ExpectedDescriptionFileName),
		[]byte(validYAML),
		0600,
	)
	require.NoError(t, err)
}
