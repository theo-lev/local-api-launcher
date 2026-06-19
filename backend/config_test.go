package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveThenLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Add(Repo{ID: "abc", Path: "/some/path"}); err != nil {
		t.Fatal(err)
	}

	s2, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	repos := s2.List()
	if len(repos) != 1 || repos[0].ID != "abc" {
		t.Fatalf("expected saved repo, got %v", repos)
	}
}

func TestRecoverFromBackupWhenTruncated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	// two saves so a .bak with the repo exists
	if err := s.Add(Repo{ID: "abc", Path: "/some/path"}); err != nil {
		t.Fatal(err)
	}
	if err := s.StorePid("abc", ProcInfo{Pid: 1234}); err != nil {
		t.Fatal(err)
	}

	// simulate a kill mid-write leaving a blank file
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}

	s2, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	repos := s2.List()
	if len(repos) != 1 || repos[0].ID != "abc" {
		t.Fatalf("expected repo restored from backup, got %v", repos)
	}
}

func TestCorruptConfigWithoutBackupFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(path); err == nil {
		t.Fatal("expected error for corrupt config without backup, got nil")
	}
}
