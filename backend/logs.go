package main

import "sync"

// LogSession holds the in-memory log buffer for one process run and fans lines
// out to any number of subscribed SSE clients.
type LogSession struct {
	mu      sync.Mutex
	lines   []string
	clients map[chan string]struct{}
}

func newLogSession() *LogSession {
	return &LogSession{clients: make(map[chan string]struct{})}
}

func (s *LogSession) append(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lines = append(s.lines, line)
	for ch := range s.clients {
		select {
		case ch <- line:
		default: // slow client — drop rather than block
		}
	}
}

// subscribe returns a snapshot of buffered lines and a live channel for future lines.
func (s *LogSession) subscribe() ([]string, chan string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan string, 256)
	s.clients[ch] = struct{}{}
	snap := make([]string, len(s.lines))
	copy(snap, s.lines)
	return snap, ch
}

func (s *LogSession) unsubscribe(ch chan string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.clients[ch]; ok {
		delete(s.clients, ch)
		close(ch)
	}
}

// closeAll signals all connected clients that the session has ended.
func (s *LogSession) closeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.clients {
		delete(s.clients, ch)
		close(ch)
	}
}
