package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCheckoutStash(t *testing.T) {
	for _, mode := range []string{"blocked", "stash", "compatible", "compatible-confirmed", "untracked-only", "untracked-collision", "invalid", "stash-failure", "switch-failure"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			git := func(args ...string) string {
				t.Helper()
				cmd := exec.Command("git", args...)
				cmd.Dir = dir
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v: %s", args, err, out)
				}
				return string(out)
			}
			write := func(name, content string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			git("init", "-b", "main")
			git("config", "user.email", "test@example.com")
			git("config", "user.name", "Test")
			write("tracked.txt", "original\n")
			git("add", ".")
			git("commit", "-m", "initial")
			git("branch", "target")
			if mode == "blocked" || mode == "stash" || mode == "stash-failure" {
				git("switch", "target")
				write("tracked.txt", "target content\n")
				git("add", "tracked.txt")
				git("commit", "-m", "target change")
				git("switch", "main")
			}
			write("tracked.txt", "staged\n")
			git("add", "tracked.txt")
			write("tracked.txt", "unstaged\n")
			write("untracked.txt", "new\n")
			if mode == "untracked-only" || mode == "untracked-collision" {
				git("restore", "--source=HEAD", "--staged", "--worktree", "tracked.txt")
			}
			if mode == "untracked-collision" {
				git("switch", "target")
				git("add", "untracked.txt")
				git("commit", "-m", "target file")
				git("switch", "main")
				write("untracked.txt", "local\n")
			}
			branch := "target"
			if mode == "invalid" {
				branch = "missing"
			}
			if mode == "stash-failure" {
				write(".git/index.lock", "locked")
			}
			if mode == "switch-failure" {
				git("worktree", "add", filepath.Join(t.TempDir(), "other"), "target")
			}
			stashName := "API Manager: main → target (2026-09-10T12:00:00.000Z)"
			err := checkoutBranch(dir, branch, mode != "blocked" && mode != "compatible" && mode != "untracked-only" && mode != "untracked-collision", stashName)
			if mode == "compatible" || mode == "compatible-confirmed" {
				if err != nil || strings.TrimSpace(git("branch", "--show-current")) != "target" || git("stash", "list") != "" {
					t.Fatalf("compatible changes should switch without stash: %v", err)
				}
				if got := git("show", ":tracked.txt"); got != "staged\n" {
					t.Fatal(got)
				}
				got, readErr := os.ReadFile(filepath.Join(dir, "tracked.txt"))
				if readErr != nil || string(got) != "unstaged\n" {
					t.Fatalf("working changes lost: %s, %v", got, readErr)
				}
				return
			}
			if mode == "untracked-only" {
				if err != nil || strings.TrimSpace(git("branch", "--show-current")) != "target" || git("stash", "list") != "" {
					t.Fatalf("untracked-only switch: %v", err)
				}
				got, readErr := os.ReadFile(filepath.Join(dir, "untracked.txt"))
				if readErr != nil || string(got) != "new\n" {
					t.Fatalf("untracked file changed: %s, %v", got, readErr)
				}
				return
			}
			if mode == "stash" {
				if err != nil {
					t.Fatal(err)
				}
				if got := strings.TrimSpace(git("branch", "--show-current")); got != "target" {
					t.Fatal(got)
				}
				if got := git("status", "--porcelain"); strings.TrimSpace(got) != "?? untracked.txt" {
					t.Fatal(got)
				}
				if got := git("stash", "list"); !strings.Contains(got, stashName) {
					t.Fatal(got)
				}
				git("switch", "main")
				git("stash", "apply", "--index")
				if got := git("show", ":tracked.txt"); got != "staged\n" {
					t.Fatal(got)
				}
				for name, want := range map[string]string{"tracked.txt": "unstaged\n", "untracked.txt": "new\n"} {
					got, err := os.ReadFile(filepath.Join(dir, name))
					if err != nil || string(got) != want {
						t.Fatalf("%s: %s, %v", name, got, err)
					}
				}
			} else {
				if err == nil {
					t.Fatal("expected failure")
				}
				if got := strings.TrimSpace(git("branch", "--show-current")); got != "main" {
					t.Fatal(got)
				}
				if mode == "blocked" {
					var dirty *DirtyWorkingTree
					if !errors.As(err, &dirty) || !reflect.DeepEqual(dirty.Files, []string{"tracked.txt"}) {
						t.Fatal(err)
					}
				}
				if got := git("stash", "list"); got != "" {
					t.Fatal(got)
				}
				if mode == "untracked-collision" {
					got, readErr := os.ReadFile(filepath.Join(dir, "untracked.txt"))
					if readErr != nil || string(got) != "local\n" {
						t.Fatalf("untracked file changed: %s, %v", got, readErr)
					}
				}
			}
		})
	}
}

func TestParseDirtyFiles(t *testing.T) {
	// " M" lines start with a significant space: trimming the raw output
	// used to eat the first letter of the first file ("pom.xml" -> "om.xml")
	out := " M pom.xml\n" +
		"M  src/main/java/App.java\n" +
		"?? untracked.txt\n" +
		"\n"
	got := parseDirtyFiles(out)
	want := []string{"pom.xml", "src/main/java/App.java"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseDirtyFilesClean(t *testing.T) {
	if got := parseDirtyFiles(""); len(got) != 0 {
		t.Fatalf("expected no files for empty output, got %v", got)
	}
}

func TestPullWithLocalChanges(t *testing.T) {
	for _, mode := range []string{"up-to-date", "compatible", "blocked", "missing-remote"} {
		t.Run(mode, func(t *testing.T) {
			upstream := t.TempDir()
			local := filepath.Join(t.TempDir(), "local")
			git := func(dir string, args ...string) string {
				t.Helper()
				cmd := exec.Command("git", args...)
				cmd.Dir = dir
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v: %s", args, err, out)
				}
				return strings.TrimSpace(string(out))
			}
			write := func(dir, name, content string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			git(upstream, "init", "-b", "main")
			git(upstream, "config", "user.name", "Test")
			git(upstream, "config", "user.email", "test@example.com")
			write(upstream, "application.yml", "original\n")
			write(upstream, "other.txt", "original\n")
			git(upstream, "add", ".")
			git(upstream, "commit", "-m", "initial")
			git(upstream, "clone", filepath.ToSlash(upstream), local)
			git(local, "config", "pull.rebase", "true")
			git(local, "config", "merge.autoStash", "true")
			git(local, "config", "rebase.autoStash", "true")
			write(local, "application.yml", "local changes\n")
			write(local, "launch.bat", "untracked\n")
			before := git(local, "rev-parse", "HEAD")
			if mode == "compatible" || mode == "blocked" {
				name := "other.txt"
				if mode == "blocked" {
					name = "application.yml"
				}
				write(upstream, name, "remote changes\n")
				git(upstream, "add", ".")
				git(upstream, "commit", "-m", "remote change")
			}
			if mode == "missing-remote" {
				git(local, "remote", "set-url", "origin", filepath.ToSlash(filepath.Join(t.TempDir(), "missing")))
			}
			err := pullBranch(local)
			var dirty *DirtyWorkingTree
			switch mode {
			case "blocked":
				if !errors.As(err, &dirty) || !reflect.DeepEqual(dirty.Files, []string{"application.yml"}) {
					t.Fatalf("expected dirty error, got %v", err)
				}
			case "missing-remote":
				if err == nil || errors.As(err, &dirty) {
					t.Fatalf("expected remote error, got %v", err)
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
			}
			wantHead := before
			if mode == "compatible" {
				wantHead = git(upstream, "rev-parse", "HEAD")
			}
			if got := git(local, "rev-parse", "HEAD"); got != wantHead {
				t.Fatalf("HEAD: %s, want %s", got, wantHead)
			}
			for name, want := range map[string]string{"application.yml": "local changes\n", "launch.bat": "untracked\n"} {
				got, readErr := os.ReadFile(filepath.Join(local, name))
				if readErr != nil || string(got) != want {
					t.Fatalf("%s changed: %s, %v", name, got, readErr)
				}
			}
			if got := git(local, "stash", "list"); got != "" {
				t.Fatalf("unexpected stash: %s", got)
			}
		})
	}
}
