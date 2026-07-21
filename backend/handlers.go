package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type RepoInfo struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	CurrentBranch string `json:"currentBranch"`
	Status        string `json:"status"`
	Port          int    `json:"port"`
	PathError     bool   `json:"pathError,omitempty"`
	Reconnected   bool   `json:"reconnected,omitempty"`
	EnvName       string `json:"envName,omitempty"`
	RunID         string `json:"runId,omitempty"`
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
	info.Status, info.RunID, _, info.EnvName, info.Reconnected = h.manager.Status(r.ID)
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

func (h *repoHandlers) pull(w http.ResponseWriter, r *http.Request, id string) {
	repo, ok := h.repoByID(w, id)
	if !ok {
		return
	}
	err := pullBranch(repo.Path)
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
	settings := h.store.GetSettings()
	envID, envName, vars := h.activeEnv()
	if err := h.manager.Start(id, repo.Path, settings.MavenPath, settings.JdkPath, envID, envName, vars); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *repoHandlers) getSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.store.GetSettings())
}

func (h *repoHandlers) putSettings(w http.ResponseWriter, r *http.Request) {
	var s Settings
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := h.store.SaveSettings(s); err != nil {
		http.Error(w, "failed to save settings", http.StatusInternalServerError)
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
	w.Header().Set("X-Accel-Buffering", "no")

	// EventSource sends Last-Event-ID on native reconnects. The UI uses one
	// explicit reconnect strategy and supplies the same run-scoped cursor here.
	afterID, cursorRunID, hasCursor := uint64(0), "", false
	cursor := r.URL.Query().Get("after")
	if cursor == "" {
		cursor = r.Header.Get("Last-Event-ID")
	}
	if cursor != "" {
		hasCursor = true
		parts := strings.SplitN(cursor, ":", 2)
		if len(parts) == 2 {
			cursorRunID = parts[0]
			var err error
			afterID, err = strconv.ParseUint(parts[1], 10, 64)
			if err != nil {
				cursorRunID = "" // malformed cursors explicitly reset
			}
		}
	}

	sub := session.subscribe(cursorRunID, afterID, hasCursor)
	defer session.unsubscribe(sub.client)
	log.Printf("log stream opened repo=%s run=%s cursor=%q retained=%d-%d", id, session.runID, cursor, sub.firstID, sub.lastID)
	defer log.Printf("log stream closed repo=%s run=%s", id, session.runID)

	if sub.reset {
		if err := writeSSEEvent(w, "session-reset", "", map[string]any{
			"runId": session.runID, "firstSequence": sub.firstID, "lastSequence": sub.lastID,
		}); err != nil {
			return
		}
	}
	if sub.gap {
		if err := writeSSEEvent(w, "retention-gap", "", map[string]any{
			"runId": session.runID, "firstSequence": sub.firstID, "lastSequence": sub.lastID,
		}); err != nil {
			return
		}
	}

	for _, entry := range sub.snapshot {
		if err := writeLogEntry(w, session.runID, entry); err != nil {
			return
		}
	}
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-sub.client.wake:
			entries, closed, gap := sub.client.take()
			if gap {
				first, last := uint64(0), uint64(0)
				if len(entries) != 0 {
					first, last = entries[0].id, entries[len(entries)-1].id
				}
				if err := writeSSEEvent(w, "retention-gap", "", map[string]any{
					"runId": session.runID, "firstSequence": first, "lastSequence": last,
				}); err != nil {
					return
				}
			}
			for _, entry := range entries {
				if err := writeLogEntry(w, session.runID, entry); err != nil {
					return
				}
			}
			if closed {
				if err := writeSSEEvent(w, "session-end", "", map[string]any{"runId": session.runID}); err != nil {
					return
				}
			}
			flusher.Flush()
			if closed {
				return
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return // client disconnected
		}
	}
}

func writeLogEntry(w io.Writer, runID string, entry logEntry) error {
	return writeSSEEvent(w, "", entry.cursor(runID), map[string]any{
		"runId": runID, "sequence": entry.id, "line": entry.line,
	})
}

// JSON keeps carriage returns, embedded newlines, and other unusual output
// escaped inside one SSE data line. It also gives the frontend the run ID on
// every entry instead of relying only on connection-level state.
func writeSSEEvent(w io.Writer, event, id string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if event != "" {
		if _, err = fmt.Fprintf(w, "event: %s\n", event); err != nil {
			return err
		}
	}
	if id != "" {
		if _, err = fmt.Fprintf(w, "id: %s\n", id); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", data)
	return err
}
