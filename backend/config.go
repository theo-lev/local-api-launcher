package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type Repo struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type Settings struct {
	MavenPath string `json:"mavenPath"`
	JdkPath   string `json:"jdkPath"`
}

type config struct {
	Repos    []Repo         `json:"repos"`
	Settings Settings       `json:"settings"`
	Pids     map[string]int `json:"pids,omitempty"`
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

func (s *Store) StorePid(id string, pid int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Pids == nil {
		s.data.Pids = make(map[string]int)
	}
	s.data.Pids[id] = pid
	return s.save()
}

func (s *Store) RemovePid(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Pids, id)
	return s.save()
}

func (s *Store) GetPids() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]int, len(s.data.Pids))
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
