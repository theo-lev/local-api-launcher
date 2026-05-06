package main

import (
	"encoding/json"
	"os"
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

func NewStore(filePath string) (*Store, error) {
	s := &Store{filePath: filePath}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	raw, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, &s.data)
}

func (s *Store) save() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, raw, 0644)
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
