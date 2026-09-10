package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type UpdateStatus struct {
	Branch   string `json:"branch"`
	Upstream string `json:"upstream"`
	Behind   int    `json:"behind"`
}

func checkRepoUpdates(ctx context.Context, repoPath string) (UpdateStatus, error) {
	status := UpdateStatus{}
	run := func(args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = repoPath
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never")
		cmd.WaitDelay = time.Second
		setProcAttr(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(out)))
		}
		return strings.TrimSpace(string(out)), nil
	}
	var err error
	status.Branch, err = run("branch", "--show-current")
	if err != nil || status.Branch == "" {
		return status, err
	}
	status.Upstream, err = run("for-each-ref", "--format=%(upstream)", "refs/heads/"+status.Branch)
	if err != nil || status.Upstream == "" {
		return status, err
	}
	if _, err = run("fetch", "--quiet"); err != nil {
		return status, err
	}
	count, err := run("rev-list", "--count", "refs/heads/"+status.Branch+".."+status.Upstream, "--")
	if err != nil {
		return status, err
	}
	status.Behind, err = strconv.Atoi(count)
	return status, err
}

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
	// Let Git preserve compatible local changes. Never auto-stash via user config.
	pullCmd := exec.Command("git", "pull", "--ff-only", "--no-rebase", "--no-autostash")
	pullCmd.Dir = repoPath
	pullCmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	if pullOut, err := pullCmd.CombinedOutput(); err != nil {
		if strings.Contains(string(pullOut), "Your local changes to the following files would be overwritten by merge") {
			files, statusErr := dirtyFiles(repoPath)
			if statusErr == nil && len(files) > 0 {
				return &DirtyWorkingTree{Files: files}
			}
		}
		return fmt.Errorf("%s", strings.TrimSpace(string(pullOut)))
	}
	return nil
}

func checkoutBranch(repoPath, branch string, stash bool, stashNames ...string) error {
	// Resolve an existing local branch before saving or modifying any work.
	validate := exec.Command("git", "show-ref", "--verify", "--", "refs/heads/"+branch)
	validate.Dir = repoPath
	if out, err := validate.CombinedOutput(); err != nil {
		return fmt.Errorf("invalid branch: %s", strings.TrimSpace(string(out)))
	}
	// Git can carry compatible local changes to the target branch. Try that
	// first, even after confirmation in case the working tree has changed.
	switchCmd := exec.Command("git", "switch", "--", branch)
	switchCmd.Dir = repoPath
	switchCmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	switchOut, switchErr := switchCmd.CombinedOutput()
	if switchErr == nil {
		return nil
	}
	// Only offer stashing for tracked changes that actually prevent checkout.
	// Locks, occupied worktrees and untracked collisions need their own remedy.
	if !strings.Contains(string(switchOut), "Your local changes to the following files would be overwritten by checkout") {
		return fmt.Errorf("%s", strings.TrimSpace(string(switchOut)))
	}
	files, err := dirtyFiles(repoPath)
	if err != nil {
		return fmt.Errorf("git status failed: %w", err)
	}
	message := ""
	if len(files) > 0 {
		if !stash {
			return &DirtyWorkingTree{Files: files}
		}
		from, err := currentBranch(repoPath)
		if err != nil {
			return err
		}
		if from == "" {
			from = "detached HEAD"
		}
		message = fmt.Sprintf("API Manager: switch %s -> %s (%s)", from, branch, time.Now().UTC().Format(time.RFC3339Nano))
		if len(stashNames) > 0 && strings.TrimSpace(stashNames[0]) != "" {
			message = stashNames[0]
		}
		save := exec.Command("git", "stash", "push", "--message", message)
		save.Dir = repoPath
		if out, err := save.CombinedOutput(); err != nil {
			return fmt.Errorf("échec du stash, branche inchangée : %s", strings.TrimSpace(string(out)))
		}
	}
	coCmd := exec.Command("git", "switch", "--", branch)
	coCmd.Dir = repoPath
	if coOut, err := coCmd.CombinedOutput(); err != nil {
		if message != "" {
			return fmt.Errorf("modifications sauvegardées dans le stash %q, mais changement impossible : %s", message, strings.TrimSpace(string(coOut)))
		}
		return fmt.Errorf("%s", strings.TrimSpace(string(coOut)))
	}
	return nil
}
