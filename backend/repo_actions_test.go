package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCheckRepoUpdates(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	local := filepath.Join(root, "local")
	other := filepath.Join(root, "other")
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git(root, "init", "--bare", "--initial-branch=main", remote)
	git(root, "clone", remote, local)
	git(local, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "initial")
	git(local, "push", "-u", "origin", "main")
	git(root, "clone", remote, other)
	check := func(want int, tracked bool) {
		t.Helper()
		status, err := checkRepoUpdates(context.Background(), local)
		if err != nil || status.Behind != want || (status.Upstream != "") != tracked {
			t.Fatalf("status = %+v, err = %v; want behind %d, tracked %v", status, err, want, tracked)
		}
	}
	check(0, true)
	git(other, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "incoming")
	git(other, "push")
	check(1, true) // Must fetch to discover this commit.
	git(local, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "local")
	check(1, true) // Diverged branches still have incoming commits.
	git(local, "switch", "-c", "untracked")
	check(0, false)
	git(local, "checkout", "--detach")
	check(0, false)
	git(local, "switch", "main")
	git(local, "remote", "set-url", "origin", filepath.Join(root, "missing.git"))
	if _, err := checkRepoUpdates(context.Background(), local); err == nil {
		t.Fatal("expected fetch failure")
	}
}

func TestIdeaCommandDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repo with spaces & symbols")
	cmd := ideaCommand(path)
	if cmd.Dir != path {
		t.Fatalf("directory = %q", cmd.Dir)
	}
	for _, arg := range cmd.Args {
		if arg == path {
			t.Fatal("repository path should be a working directory, not shell input")
		}
	}
}
