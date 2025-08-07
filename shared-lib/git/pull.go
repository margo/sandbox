package git

import (
	"fmt"
	"os"
	"time"

	goGit "github.com/go-git/go-git/v5"
	goGitConfig "github.com/go-git/go-git/v5/config"
	goGitPlumbing "github.com/go-git/go-git/v5/plumbing"
	goGitObject "github.com/go-git/go-git/v5/plumbing/object"
)

// CommitInfo represents information about a Git commit
type CommitInfo struct {
	Hash      string    `json:"hash"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	Email     string    `json:"email"`
	Timestamp time.Time `json:"timestamp"`
}

// CheckForNewCommits checks if there are new commits available in the remote repository
// compared to the local repository at the specified path.
//
// Parameters:
//   - repoPath: Path to the local Git repository (required, cannot be empty)
//   - branchName: Name of the branch to check (required, cannot be empty)
//   - auth: Optional authentication credentials for private repositories
//
// Returns:
//   - hasNewCommits: true if new commits are available, false otherwise
//   - newCommits: slice of CommitInfo representing new commits (empty if none)
//   - err: An error if the operation fails
//
// Important Notes:
//   - The local repository must already exist at the specified path
//   - This function fetches the latest refs from remote without modifying the working directory
//   - Only HTTPS-based Git URLs are supported for authentication
//
// Example:
//
//	hasNew, commits, err := CheckForNewCommits("/path/to/repo", "main", nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if hasNew {
//	    fmt.Printf("Found %d new commits\n", len(commits))
//	}
func (client *Client) CheckForNewCommits() (hasNewCommits bool, newCommits []CommitInfo, err error) {
	// Open the existing repository
	repo, err := goGit.PlainOpen(*client.repoPath)
	if err != nil {
		return false, nil, fmt.Errorf("failed to open repository at %s: %w", client.repoPath, err)
	}

	// Get current HEAD commit
	head, err := repo.Head()
	if err != nil {
		return false, nil, fmt.Errorf("failed to get repository head: %w", err)
	}
	currentCommitHash := head.Hash()

	// Fetch latest changes from remote
	err = fetchFromRemote(repo, client.auth)
	if err != nil {
		return false, nil, fmt.Errorf("failed to fetch from remote: %w", err)
	}

	// Get remote branch reference
	remoteBranchRef := fmt.Sprintf("refs/remotes/origin/%s", client.branchOrTag)
	remoteRef, err := repo.Reference(goGitPlumbing.ReferenceName(remoteBranchRef), true)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get remote branch reference: %w", err)
	}
	remoteCommitHash := remoteRef.Hash()

	// Check if remote is ahead of local
	if currentCommitHash == remoteCommitHash {
		return false, []CommitInfo{}, nil
	}

	// Get commits between current HEAD and remote HEAD
	commits, err := getCommitsBetween(repo, currentCommitHash, remoteCommitHash)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get commits between hashes: %w", err)
	}

	return len(commits) > 0, commits, nil
}

// PullLatestChanges pulls the latest changes from the remote repository to the local repository.
//
// Parameters:
//   - repoPath: Path to the local Git repository (required, cannot be empty)
//   - branchName: Name of the branch to pull (required, cannot be empty)
//   - auth: Optional authentication credentials for private repositories
//
// Returns:
//   - pulledCommits: slice of CommitInfo representing the commits that were pulled
//   - err: An error if the pull operation fails
//
// Important Notes:
//   - The local repository must already exist at the specified path
//   - This function performs a fast-forward merge only
//   - If there are local uncommitted changes, the pull may fail
//   - Only HTTPS-based Git URLs are supported for authentication
//   - Progress information is written to os.Stdout during pulling
//
// Example:
//
//	commits, err := PullLatestChanges("/path/to/repo", "main", nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Pulled %d new commits\n", len(commits))
func PullLatestChanges(repoPath, branchName string, auth *Auth) (pulledCommits []CommitInfo, err error) {
	// Validate inputs
	if repoPath == "" {
		return nil, fmt.Errorf("repository path cannot be empty")
	}
	if branchName == "" {
		return nil, fmt.Errorf("branch name cannot be empty")
	}

	// Open the existing repository
	repo, err := goGit.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository at %s: %w", repoPath, err)
	}

	// Get current HEAD commit before pull
	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get repository head: %w", err)
	}
	beforePullHash := head.Hash()

	// Get working tree
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get working tree: %w", err)
	}

	// Prepare pull options
	pullOptions := &goGit.PullOptions{
		ReferenceName: goGitPlumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branchName)),
		SingleBranch:  true,
		Progress:      os.Stdout,
	}

	// Set authentication if provided
	if auth != nil {
		authMethod, err := getAuthMethod("", auth) // URL not needed for existing repo
		if err != nil {
			return nil, fmt.Errorf("failed to setup authentication: %w", err)
		}
		pullOptions.Auth = authMethod
	}

	// Perform the pull
	err = worktree.Pull(pullOptions)
	if err != nil {
		if err == goGit.NoErrAlreadyUpToDate {
			fmt.Println("Repository is already up to date")
			return []CommitInfo{}, nil
		}
		return nil, fmt.Errorf("failed to pull changes: %w", err)
	}

	// Get new HEAD commit after pull
	newHead, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get repository head after pull: %w", err)
	}
	afterPullHash := newHead.Hash()

	// Get commits that were pulled
	if beforePullHash != afterPullHash {
		commits, err := getCommitsBetween(repo, beforePullHash, afterPullHash)
		if err != nil {
			return nil, fmt.Errorf("failed to get pulled commits: %w", err)
		}

		fmt.Printf("Successfully pulled %d new commits\n", len(commits))
		fmt.Printf("Updated to commit: %s\n", afterPullHash)
		return commits, nil
	}

	return []CommitInfo{}, nil
}

// GetLatestCommitInfo retrieves information about the latest commit in the specified branch.
//
// Parameters:
//   - repoPath: Path to the local Git repository (required, cannot be empty)
//   - branchName: Name of the branch to check (optional, if empty uses current HEAD)
//
// Returns:
//   - commitInfo: Information about the latest commit
//   - err: An error if the operation fails
//
// Example:
//
//	info, err := GetLatestCommitInfo("/path/to/repo", "main")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Latest commit: %s by %s\n", info.Hash, info.Author)
func GetLatestCommitInfo(repoPath, branchName string) (commitInfo CommitInfo, err error) {
	if repoPath == "" {
		return CommitInfo{}, fmt.Errorf("repository path cannot be empty")
	}

	repo, err := goGit.PlainOpen(repoPath)
	if err != nil {
		return CommitInfo{}, fmt.Errorf("failed to open repository: %w", err)
	}

	var ref *goGitPlumbing.Reference
	if branchName != "" {
		ref, err = repo.Reference(goGitPlumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branchName)), true)
	} else {
		ref, err = repo.Head()
	}

	if err != nil {
		return CommitInfo{}, fmt.Errorf("failed to get reference: %w", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return CommitInfo{}, fmt.Errorf("failed to get commit object: %w", err)
	}

	return CommitInfo{
		Hash:      commit.Hash.String(),
		Message:   commit.Message,
		Author:    commit.Author.Name,
		Email:     commit.Author.Email,
		Timestamp: commit.Author.When,
	}, nil
}

// Helper function to fetch from remote
func fetchFromRemote(repo *goGit.Repository, auth *Auth) error {
	fetchOptions := &goGit.FetchOptions{
		RefSpecs: []goGitConfig.RefSpec{"refs/heads/*:refs/remotes/origin/*"},
		Progress: os.Stdout,
	}

	if auth != nil {
		authMethod, err := getAuthMethod("", auth)
		if err != nil {
			return fmt.Errorf("failed to setup authentication: %w", err)
		}
		fetchOptions.Auth = authMethod
	}

	err := repo.Fetch(fetchOptions)
	if err != nil && err != goGit.NoErrAlreadyUpToDate {
		return err
	}
	return nil
}

// Helper function to get commits between two hashes
func getCommitsBetween(repo *goGit.Repository, fromHash, toHash goGitPlumbing.Hash) ([]CommitInfo, error) {
	var commits []CommitInfo

	// Get commit iterator from the target hash
	commitIter, err := repo.Log(&goGit.LogOptions{From: toHash})
	if err != nil {
		return nil, err
	}
	defer commitIter.Close()

	// Iterate through commits until we reach the fromHash
	err = commitIter.ForEach(func(commit *goGitObject.Commit) error {
		if commit.Hash == fromHash {
			return fmt.Errorf("reached base commit") // Stop iteration
		}

		commits = append(commits, CommitInfo{
			Hash:      commit.Hash.String(),
			Message:   commit.Message,
			Author:    commit.Author.Name,
			Email:     commit.Author.Email,
			Timestamp: commit.Author.When,
		})
		return nil
	})

	if err != nil && err.Error() != "reached base commit" {
		return nil, err
	}

	return commits, nil
}
