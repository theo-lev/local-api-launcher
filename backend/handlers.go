package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
)

type RepoInfo struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	CurrentBranch string `json:"currentBranch"`
	Status        string `json:"status"`
	Port          int    `json:"port"`
	PathError     bool   `json:"pathError,omitempty"`
}

type repoHandlers struct {
	store   *Store
	manager *ProcessManager
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *repoHandlers) enrich(r Repo) RepoInfo {
	info := RepoInfo{ID: r.ID, Path: r.Path, Status: "stopped"}
	if h.manager.IsRunning(r.ID) {
		info.Status = "running"
	}
	if _, err := os.Stat(r.Path); os.IsNotExist(err) {
		info.PathError = true
		return info
	}
	info.CurrentBranch, _ = currentBranch(r.Path)
	info.Port = readPort(r.Path)
	return info
}

// repoByID looks up a stored Repo and returns 404 if not found.
// Returns ("", false) and writes the error itself on failure.
func (h *repoHandlers) repoByID(w http.ResponseWriter, id string) (Repo, bool) {
	for _, r := range h.store.List() {
		if r.ID == id {
			return r, true
		}
	}
	http.Error(w, "repo not found", http.StatusNotFound)
	return Repo{}, false
}

// --- collection handlers ---

func (h *repoHandlers) list(w http.ResponseWriter, r *http.Request) {
	repos := h.store.List()
	infos := make([]RepoInfo, len(repos))
	for i, repo := range repos {
		infos[i] = h.enrich(repo)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(infos)
}

func (h *repoHandlers) add(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Path == "" {
		http.Error(w, "missing or invalid path", http.StatusBadRequest)
		return
	}
	if _, err := os.Stat(body.Path); os.IsNotExist(err) {
		http.Error(w, "path does not exist on disk", http.StatusBadRequest)
		return
	}
	repo := Repo{ID: newID(), Path: body.Path}
	if err := h.store.Add(repo); err != nil {
		http.Error(w, "failed to save config", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(h.enrich(repo))
}

// --- per-repo handlers (id passed by router) ---

func (h *repoHandlers) remove(w http.ResponseWriter, r *http.Request, id string) {
	found, err := h.store.Remove(id)
	if err != nil {
		http.Error(w, "failed to save config", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "repo not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *repoHandlers) fetch(w http.ResponseWriter, r *http.Request, id string) {
	repo, ok := h.repoByID(w, id)
	if !ok {
		return
	}
	if err := fetchRepo(repo.Path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *repoHandlers) branches(w http.ResponseWriter, r *http.Request, id string) {
	repo, ok := h.repoByID(w, id)
	if !ok {
		return
	}
	list, err := listBranches(repo.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (h *repoHandlers) checkout(w http.ResponseWriter, r *http.Request, id string) {
	repo, ok := h.repoByID(w, id)
	if !ok {
		return
	}
	var body struct {
		Branch string `json:"branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Branch == "" {
		http.Error(w, "missing branch", http.StatusBadRequest)
		return
	}
	err := checkoutBranch(repo.Path, body.Branch)
	if err == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var dirty *DirtyWorkingTree
	if errors.As(err, &dirty) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]any{
			"error": "dirty",
			"files": dirty.Files,
		})
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (h *repoHandlers) start(w http.ResponseWriter, r *http.Request, id string) {
	repo, ok := h.repoByID(w, id)
	if !ok {
		return
	}
	if err := h.manager.Start(id, repo.Path); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *repoHandlers) stop(w http.ResponseWriter, r *http.Request, id string) {
	if _, ok := h.repoByID(w, id); !ok {
		return
	}
	if err := h.manager.Stop(id); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *repoHandlers) logs(w http.ResponseWriter, r *http.Request, id string) {
	if _, ok := h.repoByID(w, id); !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	session := h.manager.GetSession(id)
	if session == nil {
		http.Error(w, "no active session", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	snapshot, ch := session.subscribe()
	defer session.unsubscribe(ch)

	for _, line := range snapshot {
		fmt.Fprintf(w, "data: %s\n\n", line)
	}
	flusher.Flush()

	for {
		select {
		case line, ok := <-ch:
			if !ok {
				return // session closed (process stopped)
			}
			fmt.Fprintf(w, "data: %s\n\n", line)
			flusher.Flush()
		case <-r.Context().Done():
			return // client disconnected
		}
	}
}
