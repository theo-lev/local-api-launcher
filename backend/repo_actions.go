package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

func (h *repoHandlers) updates(w http.ResponseWriter, r *http.Request, id string) {
	repo, ok := h.repoByID(w, id)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	status, err := checkRepoUpdates(ctx, repo.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func ideaCommand(repoPath string) *exec.Cmd {
	cmd := exec.Command("idea", ".")
	if runtime.GOOS == "windows" {
		// Also support idea.cmd / idea.bat launchers on PATH. No user input
		// is interpolated into the shell command; the directory is set below.
		cmd = exec.Command("cmd.exe", "/d", "/c", "idea .")
	}
	cmd.Dir = repoPath
	setProcAttr(cmd)
	return cmd
}

func (h *repoHandlers) openIdea(w http.ResponseWriter, r *http.Request, id string) {
	repo, ok := h.repoByID(w, id)
	if !ok {
		return
	}
	cmd := ideaCommand(repo.Path)
	if err := cmd.Start(); err != nil {
		http.Error(w, fmt.Sprintf("Could not open IntelliJ: %v. Check that idea is on PATH.", err), http.StatusInternalServerError)
		return
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			http.Error(w, "Could not open IntelliJ. Check that idea is on PATH and runs from this folder.", http.StatusInternalServerError)
			return
		}
	case <-time.After(time.Second):
		// Some launchers stay attached to the IDE; reap them when it closes.
	}
	w.WriteHeader(http.StatusNoContent)
}
