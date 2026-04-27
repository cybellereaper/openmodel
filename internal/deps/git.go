package deps

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrGitMissing is returned when the local git executable is unavailable.
var ErrGitMissing = errors.New("git is required to download PureLang dependencies")

// GitInstalled reports whether git is found in $PATH.
func GitInstalled() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// Clone clones a repository to dest.
func Clone(url string, dest string) error {
	if !GitInstalled() {
		return ErrGitMissing
	}
	cmd := exec.Command("git", "clone", url, dest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone %s failed: %v\n%s", url, err, string(out))
	}
	return nil
}

// Checkout checks out a ref in repoDir.
func Checkout(repoDir string, ref string) error {
	if !GitInstalled() {
		return ErrGitMissing
	}
	cmd := exec.Command("git", "checkout", ref)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout %s in %s failed: %v\n%s", ref, repoDir, err, string(out))
	}
	return nil
}

// Pull pulls the current branch in repoDir.
func Pull(repoDir string) error {
	if !GitInstalled() {
		return ErrGitMissing
	}
	cmd := exec.Command("git", "pull", "--ff-only")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull in %s failed: %v\n%s", repoDir, err, string(out))
	}
	return nil
}

// CurrentCommit returns the resolved HEAD commit hash for repoDir.
func CurrentCommit(repoDir string) (string, error) {
	if !GitInstalled() {
		return "", ErrGitMissing
	}
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse in %s failed: %v", repoDir, err)
	}
	return strings.TrimSpace(string(out)), nil
}
