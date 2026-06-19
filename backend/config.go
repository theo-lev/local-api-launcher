package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	errNameTaken   = errors.New("an environment with that name already exists")
	errEnvNotFound = errors.New("environment not found")
)

type Repo struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type Settings struct {
	MavenPath string `json:"mavenPath"`
	JdkPath   string `json:"jdkPath"`
}

// EnvSet is a named, global collection of environment variables. Vars is kept
// as raw dotenv-style text (KEY=VALUE per line, # comments) so it round-trips
// the editor textarea exactly; it is parsed only when a process is launched.
type EnvSet struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Vars string `json:"vars"`
}

// ProcInfo records a running process and the environment it was launched under,
// so the "running under" tag survives an app restart + reconnect. EnvName is a
// snapshot used for display when EnvID no longer resolves (e.g. set deleted).
type ProcInfo struct {
	Pid     int    `json:"pid"`
	EnvID   string `json:"envId,omitempty"`
	EnvName string `json:"envName,omitempty"`
}

// ProcMap tolerates the legacy on-disk format where pids was a map[string]int,
// so existing config.json files keep loading after the upgrade.
type ProcMap map[string]ProcInfo

func (p *ProcMap) UnmarshalJSON(data []byte) error {
	m := map[string]ProcInfo{}
	if err := json.Unmarshal(data, &m); err == nil {
		*p = m
		return nil
	}
	var legacy map[string]int
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}
	m = make(map[string]ProcInfo, len(legacy))
	for k, v := range legacy {
		m[k] = ProcInfo{Pid: v}
	}
	*p = m
	return nil
}

type config struct {
	Repos       []Repo   `json:"repos"`
	Settings    Settings `json:"settings"`
	EnvSets     []EnvSet `json:"envSets,omitempty"`
	ActiveEnvID string   `json:"activeEnvId,omitempty"`
	Pids        ProcMap  `json:"pids,omitempty"`
}

type Store struct {
	mu       sync.RWMutex
	data     config
	filePath string
}

// configPath keeps existing setups working (config.json in the working
// directory) but anchors fresh ones next to the executable, so launching
// from another directory doesn't silently start with an empty config.
func configPath() string {
	if _, err := os.Stat("config.json"); err == nil {
		return "config.json"
	}
	exe, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(filepath.Dir(exe), "config.json")
}

func NewStore(filePath string) (*Store, error) {
	s := &Store{filePath: filePath}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	err := s.loadFile(s.filePath)
	if err == nil {
		return nil
	}
	if bakErr := s.loadFile(s.filePath + ".bak"); bakErr == nil {
		log.Printf("config file unusable (%v), restored from %s.bak", err, s.filePath)
		return s.save()
	}
	if os.IsNotExist(err) {
		return err // fresh install, tolerated by NewStore
	}
	return fmt.Errorf("config file is corrupt and no usable backup exists (delete %s to start fresh): %w", s.filePath, err)
}

func (s *Store) loadFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return errors.New("file is empty")
	}
	return json.Unmarshal(raw, &s.data)
}

// save writes to a temp file and renames it into place: a process killed
// mid-save can no longer leave a truncated config. The previous good
// version is kept as .bak for recovery.
func (s *Store) save() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0644); err != nil {
		return err
	}
	if old, err := os.ReadFile(s.filePath); err == nil && len(bytes.TrimSpace(old)) > 0 {
		os.WriteFile(s.filePath+".bak", old, 0644)
	}
	return os.Rename(tmp, s.filePath)
}

func (s *Store) List() []Repo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Repo, len(s.data.Repos))
	copy(out, s.data.Repos)
	return out
}

func (s *Store) Add(r Repo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Repos = append(s.data.Repos, r)
	return s.save()
}

func (s *Store) Remove(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.data.Repos {
		if r.ID == id {
			s.data.Repos = append(s.data.Repos[:i], s.data.Repos[i+1:]...)
			return true, s.save()
		}
	}
	return false, nil
}

func (s *Store) StorePid(id string, info ProcInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Pids == nil {
		s.data.Pids = make(ProcMap)
	}
	s.data.Pids[id] = info
	return s.save()
}

func (s *Store) RemovePid(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Pids, id)
	return s.save()
}

func (s *Store) GetPids() map[string]ProcInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]ProcInfo, len(s.data.Pids))
	for k, v := range s.data.Pids {
		out[k] = v
	}
	return out
}

func (s *Store) GetSettings() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Settings
}

func (s *Store) SaveSettings(settings Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Settings = settings
	return s.save()
}

func (s *Store) ListEnvSets() []EnvSet {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]EnvSet, len(s.data.EnvSets))
	copy(out, s.data.EnvSets)
	return out
}

func (s *Store) GetEnvSet(id string) (EnvSet, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.data.EnvSets {
		if e.ID == id {
			return e, true
		}
	}
	return EnvSet{}, false
}

// nameTaken reports whether some other set (id != exceptID) already uses name,
// compared case-insensitively. Caller must hold the lock.
func (s *Store) nameTaken(name, exceptID string) bool {
	for _, e := range s.data.EnvSets {
		if e.ID != exceptID && strings.EqualFold(e.Name, name) {
			return true
		}
	}
	return false
}

// AddEnvSet returns errNameTaken if the name collides with an existing set.
func (s *Store) AddEnvSet(e EnvSet) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nameTaken(e.Name, "") {
		return errNameTaken
	}
	s.data.EnvSets = append(s.data.EnvSets, e)
	return s.save()
}

// UpdateEnvSet returns (false, nil) if no set has the id, or errNameTaken on a
// name collision with a different set.
func (s *Store) UpdateEnvSet(e EnvSet) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nameTaken(e.Name, e.ID) {
		return false, errNameTaken
	}
	for i, existing := range s.data.EnvSets {
		if existing.ID == e.ID {
			s.data.EnvSets[i].Name = e.Name
			s.data.EnvSets[i].Vars = e.Vars
			return true, s.save()
		}
	}
	return false, nil
}

// RemoveEnvSet deletes the set and clears the active selection if it pointed at
// the deleted set, falling back to None.
func (s *Store) RemoveEnvSet(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.data.EnvSets {
		if e.ID == id {
			s.data.EnvSets = append(s.data.EnvSets[:i], s.data.EnvSets[i+1:]...)
			if s.data.ActiveEnvID == id {
				s.data.ActiveEnvID = ""
			}
			return true, s.save()
		}
	}
	return false, nil
}

func (s *Store) GetActiveEnvID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.ActiveEnvID
}

// SetActiveEnvID accepts "" (None) or the id of an existing set; an unknown id
// returns errEnvNotFound.
func (s *Store) SetActiveEnvID(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id != "" {
		found := false
		for _, e := range s.data.EnvSets {
			if e.ID == id {
				found = true
				break
			}
		}
		if !found {
			return errEnvNotFound
		}
	}
	s.data.ActiveEnvID = id
	return s.save()
}
