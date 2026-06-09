package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func currentBranch(repoPath string) (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func fetchRepo(repoPath string) error {
	cmd := exec.Command("git", "fetch")
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func listBranches(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "branch")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, line := range strings.Split(string(out), "\n") {
		name := strings.TrimSpace(strings.TrimPrefix(line, "*"))
		if name != "" {
			branches = append(branches, name)
		}
	}
	return branches, nil
}

// DirtyWorkingTree is returned by checkoutBranch when uncommitted changes block the switch.
type DirtyWorkingTree struct {
	Files []string
}

func (e *DirtyWorkingTree) Error() string { return "working tree has uncommitted changes" }

// parseDirtyFiles extracts tracked, changed file paths from
// `git status --porcelain` output. Each line is "XY path" where XY is a
// two-character status code (the first may be a significant space, so the
// output must not be trimmed before splitting). Untracked files (??) don't
// block pull/checkout and are skipped.
func parseDirtyFiles(out string) []string {
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 || strings.HasPrefix(line, "??") {
			continue
		}
		files = append(files, strings.TrimSpace(line[3:]))
	}
	return files
}

func dirtyFiles(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w", err)
	}
	return parseDirtyFiles(string(out)), nil
}

func pullBranch(repoPath string) error {
	files, err := dirtyFiles(repoPath)
	if err != nil {
		return err
	}
	if len(files) > 0 {
		return &DirtyWorkingTree{Files: files}
	}
	pullCmd := exec.Command("git", "pull", "--ff-only")
	pullCmd.Dir = repoPath
	if pullOut, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(pullOut)))
	}
	return nil
}

func checkoutBranch(repoPath, branch string) error {
	files, err := dirtyFiles(repoPath)
	if err != nil {
		return err
	}
	if len(files) > 0 {
		return &DirtyWorkingTree{Files: files}
	}
	coCmd := exec.Command("git", "checkout", branch)
	coCmd.Dir = repoPath
	if coOut, err := coCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(coOut)))
	}
	return nil
}
