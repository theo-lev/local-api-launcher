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

type config struct {
	Repos []Repo `json:"repos"`
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
