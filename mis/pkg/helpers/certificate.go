package helpers

import (
	"fmt"
	"os"
	"path/filepath"
)

/*
CreateBundleFile reads one or more certificate files, concatenates their
contents in the order provided, and writes the result to a temporary file
created in the current working directory.

The generated file can be used as a certificate bundle or trust chain.

Parameters:
  - certPaths: One or more certificate file paths.

Returns:
  - Absolute path to the generated bundle file.
  - nil error on success.
  - Non-nil error if any certificate cannot be read, the temporary
    file cannot be created, or the bundle cannot be written.

Example:

	bundlePath, err := CreateBundleFile(
		"./root-ca.crt",
		"./intermediate-ca.crt",
	)
	if err != nil {
		return err
	}

	defer os.Remove(bundlePath)
*/
func CreateBundleFile(certPaths ...string) (string, error) {
	if len(certPaths) == 0 {
		return "", fmt.Errorf("at least one certificate path must be provided")
	}

	// Create the temporary file in the current working directory.
	tmpFile, err := os.CreateTemp(".", "cert-bundle-*.crt")
	if err != nil {
		return "", fmt.Errorf("create temporary bundle file: %w", err)
	}

	// Ensure the file is closed before returning.
	defer tmpFile.Close()

	for _, certPath := range certPaths {
		// Read certificate content.
		certData, err := os.ReadFile(certPath)
		if err != nil {
			os.Remove(tmpFile.Name())
			return "", fmt.Errorf("read certificate %q: %w", certPath, err)
		}

		// Write certificate content to the bundle.
		if _, err := tmpFile.Write(certData); err != nil {
			os.Remove(tmpFile.Name())
			return "", fmt.Errorf(
				"write certificate %q to bundle: %w",
				certPath,
				err,
			)
		}

		// Add a newline between certificates if needed.
		if len(certData) > 0 && certData[len(certData)-1] != '\n' {
			if _, err := tmpFile.WriteString("\n"); err != nil {
				os.Remove(tmpFile.Name())
				return "", fmt.Errorf(
					"write separator after %q: %w",
					certPath,
					err,
				)
			}
		}
	}

	absPath, err := filepath.Abs(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("resolve absolute bundle path: %w", err)
	}

	return absPath, nil
}
