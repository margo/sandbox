package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// GitAuth holds authentication credentials
type GitAuth struct {
	Username string
	Token    string
}

// ReadFromGit clones a repository using go-git library
func ReadFromGit(url, branchName string) (dirPath string, err error) {
	return ReadFromGitWithAuth(url, branchName, nil)
}

// ReadFromGitWithAuth clones a repository with optional authentication
func ReadFromGitWithAuth(url string, branchName string, auth *GitAuth) (dirPath string, err error) {
	// Validate URL
	if url == "" {
		return "", fmt.Errorf("git URL cannot be empty")
	}

	// Extract repository name from URL for directory naming
	repoName := extractRepoName(url)
	if repoName == "" {
		return "", fmt.Errorf("invalid git URL format")
	}

	// Create temporary directory for cloning
	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("margo-git-%d", time.Now().Unix()))
	cloneDir := filepath.Join(tempDir, repoName)

	// Ensure temp directory exists
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Clean up on function exit
	defer func() {
		os.RemoveAll(tempDir)
	}()

	// Prepare clone options
	cloneOptions := &git.CloneOptions{
		URL:           url,
		Progress:      os.Stdout,
		ReferenceName: plumbing.ReferenceName(branchName),
		SingleBranch:  true,
	}

	// Set authentication if provided
	if auth != nil {
		authMethod, err := getAuthMethod(url, auth)
		if err != nil {
			return "", fmt.Errorf("failed to setup authentication: %w", err)
		}
		cloneOptions.Auth = authMethod
	}

	// Clone the repository
	repo, err := git.PlainClone(cloneDir, false, cloneOptions)
	if err != nil {
		return "", fmt.Errorf("failed to clone repository from %s: %w", url, err)
	}

	// Verify the clone was successful
	if _, err := os.Stat(cloneDir); os.IsNotExist(err) {
		return "", fmt.Errorf("repository clone failed: directory not found")
	}

	// Get repository info
	head, err := repo.Head()
	if err != nil {
		return cloneDir, fmt.Errorf("failed to get repository head: %w", err)
	}

	fmt.Printf("Successfully cloned repository to: %s\n", cloneDir)
	fmt.Printf("Current commit: %s\n", head.Hash())
	return cloneDir, nil
}

// getAuthMethod returns appropriate authentication method based on URL and auth info
func getAuthMethod(url string, auth *GitAuth) (transport.AuthMethod, error) {
	// SSH authentication
	if strings.HasPrefix(url, "git@") || strings.Contains(url, "ssh://") {
		return nil, fmt.Errorf("only https based git is supported")
	}

	// HTTPS authentication
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		if auth.Username != "" && auth.Token != "" {
			return &http.BasicAuth{
				Username: auth.Username,
				Password: auth.Token,
			}, nil
		}
	}

	return nil, nil
}

// extractRepoName extracts repository name from Git URL
func extractRepoName(url string) string {
	// Handle different URL formats:
	// https://github.com/user/repo.git
	// https://github.com/user/repo
	// git@github.com:user/repo.git

	url = strings.TrimSuffix(url, ".git")

	// For HTTPS URLs
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		parts := strings.Split(url, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	// For SSH URLs
	if strings.Contains(url, ":") && !strings.Contains(url, "://") {
		parts := strings.Split(url, ":")
		if len(parts) > 1 {
			pathParts := strings.Split(parts[1], "/")
			if len(pathParts) > 0 {
				return pathParts[len(pathParts)-1]
			}
		}
	}

	return ""
}

// CloneToDirectory clones repository to a specific directory (doesn't clean up)
func CloneToDirectory(url, targetDir string, auth *GitAuth) error {
	if url == "" {
		return fmt.Errorf("git URL cannot be empty")
	}

	if targetDir == "" {
		return fmt.Errorf("target directory cannot be empty")
	}

	// Prepare clone options
	cloneOptions := &git.CloneOptions{
		URL:      url,
		Progress: os.Stdout,
	}

	// Set authentication if provided
	if auth != nil {
		authMethod, err := getAuthMethod(url, auth)
		if err != nil {
			return fmt.Errorf("failed to setup authentication: %w", err)
		}
		cloneOptions.Auth = authMethod
	}

	// Clone the repository
	_, err := git.PlainClone(targetDir, false, cloneOptions)
	if err != nil {
		return fmt.Errorf("failed to clone repository from %s to %s: %w", url, targetDir, err)
	}

	fmt.Printf("Successfully cloned repository to: %s\n", targetDir)
	return nil
}
