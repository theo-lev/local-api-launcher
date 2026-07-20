package main

import "sync"

const maxLogLines = 2000

type logEntry struct {
	id   uint64
	line string
}

type LogSession struct {
	mu      sync.Mutex
	lines   []logEntry
	clients map[*logClient]struct{}
	closed  bool
	nextID  uint64
}

// logClient is one subscriber's outbound queue. Lines are appended without
// blocking the producer and drained in batches by the SSE handler, so a slow
// reader can never stall the log pipeline (and through it, the child process)
// nor silently drop lines the way a small fixed-size channel did.
type logClient struct {
	mu     sync.Mutex
	buf    []logEntry
	closed bool
	wake   chan struct{} // cap 1; signals "lines buffered or session closed"
}

func newLogSession() *LogSession {
	return &LogSession{clients: make(map[*logClient]struct{})}
}

func (s *LogSession) append(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	entry := logEntry{id: s.nextID, line: line}
	s.lines = append(s.lines, entry)
	if len(s.lines) > maxLogLines {
		s.lines = s.lines[len(s.lines)-maxLogLines:]
	}
	for c := range s.clients {
		c.push(entry)
	}
}

// push appends a line to the client's queue and wakes its reader. It never
// blocks. The queue is bounded by maxLogLines (matching the session's own
// retention): a reader that falls that far behind drops its oldest lines,
// which is no worse than what a fresh reconnect would have replayed anyway.
func (c *logClient) push(entry logEntry) {
	c.mu.Lock()
	c.buf = append(c.buf, entry)
	if len(c.buf) > maxLogLines {
		c.buf = c.buf[len(c.buf)-maxLogLines:]
	}
	c.mu.Unlock()
	c.signal()
}

func (c *logClient) signal() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// take drains all buffered lines and reports whether the session has closed.
func (c *logClient) take() (lines []logEntry, closed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	lines, c.buf = c.buf, nil
	return lines, c.closed
}

// subscribe returns buffered entries newer than afterID and a live client for
// future entries. The snapshot and registration happen atomically under the
// lock, so no entry can slip between the two (no gaps, no duplicates). A zero
// cursor is the initial connection and receives the complete retained history.
func (s *LogSession) subscribe(afterID uint64) ([]logEntry, *logClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := &logClient{wake: make(chan struct{}, 1)}
	first := len(s.lines)
	for i, entry := range s.lines {
		if entry.id > afterID {
			first = i
			break
		}
	}
	snap := make([]logEntry, len(s.lines)-first)
	copy(snap, s.lines[first:])
	if s.closed {
		c.closed = true
		c.signal() // let the handler flush the snapshot, then exit
	} else {
		s.clients[c] = struct{}{}
	}
	return snap, c
}

func (s *LogSession) unsubscribe(c *logClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, c)
}

// closeAll signals all connected clients that the session has ended, leaving
// any still-buffered lines for the handler to flush before it returns.
func (s *LogSession) closeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	for c := range s.clients {
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()
		c.signal()
		delete(s.clients, c)
	}
}
