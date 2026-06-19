package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// parseEnvVars turns dotenv-style text into a []string of "KEY=VALUE" entries
// suitable for exec.Cmd.Env. Blank lines and lines starting with '#' are
// skipped, as are lines with no '='. The key is trimmed; the value is taken
// verbatim (no quote-stripping). Duplicate keys are left in source order, so a
// later definition wins once handed to the OS — same rule as the environment.
func parseEnvVars(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		if key == "" {
			continue
		}
		out = append(out, key+"="+line[eq+1:])
	}
	return out
}

type envSetBody struct {
	Name string `json:"name"`
	Vars string `json:"vars"`
}

// activeEnv resolves the active environment to its id, name and parsed vars.
// An empty active selection — or one pointing at a since-deleted set — yields
// the None case: empty id/name and no vars.
func (h *repoHandlers) activeEnv() (id, name string, vars []string) {
	id = h.store.GetActiveEnvID()
	if id == "" {
		return "", "", nil
	}
	set, ok := h.store.GetEnvSet(id)
	if !ok {
		return "", "", nil
	}
	return set.ID, set.Name, parseEnvVars(set.Vars)
}

func (h *repoHandlers) listEnvs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"environments": h.store.ListEnvSets(),
		"activeId":     h.store.GetActiveEnvID(),
	})
}

func (h *repoHandlers) createEnv(w http.ResponseWriter, r *http.Request) {
	var body envSetBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	set := EnvSet{ID: newID(), Name: body.Name, Vars: body.Vars}
	if err := h.store.AddEnvSet(set); err != nil {
		writeEnvErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(set)
}

func (h *repoHandlers) updateEnv(w http.ResponseWriter, r *http.Request, id string) {
	var body envSetBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	found, err := h.store.UpdateEnvSet(EnvSet{ID: id, Name: body.Name, Vars: body.Vars})
	if err != nil {
		writeEnvErr(w, err)
		return
	}
	if !found {
		http.Error(w, "environment not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *repoHandlers) deleteEnv(w http.ResponseWriter, r *http.Request, id string) {
	found, err := h.store.RemoveEnvSet(id)
	if err != nil {
		http.Error(w, "failed to save config", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "environment not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *repoHandlers) setActiveEnv(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := h.store.SetActiveEnvID(body.ID); err != nil {
		writeEnvErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeEnvErr maps the store's sentinel errors to HTTP status codes.
func writeEnvErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errNameTaken):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, errEnvNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	default:
		http.Error(w, "failed to save config", http.StatusInternalServerError)
	}
}
