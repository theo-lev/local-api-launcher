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

func checkoutBranch(repoPath, branch string) error {
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = repoPath
	out, err := statusCmd.Output()
	if err != nil {
		return fmt.Errorf("git status failed: %w", err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" || strings.HasPrefix(line, "??") {
			continue
		}
		if len(line) > 3 {
			files = append(files, strings.TrimSpace(line[3:]))
		}
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
