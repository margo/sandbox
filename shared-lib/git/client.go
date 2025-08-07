package git

import (
	"fmt"
	"os"
	"path/filepath"
)

type Client struct {
	url         string
	branchOrTag string
	// currentPath: a random path will be decide at the time of cloning if the currentRepoPath is not provided by the user
	repoPath *string
	auth     *Auth
}

func NewClient(auth *Auth, url, branchOrTagName string, outputPath *string) (*Client, error) {
	// Validate URL
	if url == "" {
		return nil, fmt.Errorf("git URL cannot be empty")
	}
	// Validate branch name
	if branchOrTagName == "" {
		return nil, fmt.Errorf("git branchOrTagName cannot be empty")
	}

	// Check if outputPath actually exists on the disk and is valid
	if outputPath != nil {
		// Check if the path exists
		if _, err := os.Stat(*outputPath); err != nil {
			if os.IsNotExist(err) {
				// Try to create the directory if it doesn't exist
				if err := os.MkdirAll(*outputPath, 0755); err != nil {
					return nil, fmt.Errorf("output path does not exist and cannot be created: %w", err)
				}
			} else {
				return nil, fmt.Errorf("failed to access output path: %w", err)
			}
		}

		// Check if the path is a directory
		fileInfo, err := os.Stat(*outputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get file info for output path: %w", err)
		}
		if !fileInfo.IsDir() {
			return nil, fmt.Errorf("output path must be a directory, not a file")
		}

		// Check if the directory is writable
		testFile := filepath.Join(*outputPath, ".write_test")
		if file, err := os.Create(testFile); err != nil {
			return nil, fmt.Errorf("output path is not writable: %w", err)
		} else {
			file.Close()
			os.Remove(testFile) // Clean up test file
		}

		// Convert to absolute path for consistency
		absPath, err := filepath.Abs(*outputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path: %w", err)
		}
		outputPath = &absPath
	}

	return &Client{
		auth:        auth,
		url:         url,
		branchOrTag: branchOrTagName,
		repoPath:    outputPath,
	}, nil
}
