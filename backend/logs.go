package main

import (
	"fmt"
	"log"
	"sync"
)

const maxLogLines = 2000

type logEntry struct {
	id   uint64
	line string
}

func (e logEntry) cursor(runID string) string {
	return fmt.Sprintf("%s:%d", runID, e.id)
}

type LogSession struct {
	mu      sync.Mutex
	runID   string
	lines   []logEntry
	clients map[*logClient]struct{}
	closed  bool
	nextID  uint64
}

// logClient is one subscriber's bounded outbound queue. If it overflows, gap
// is set so the handler can explicitly tell the browser that older entries
// were discarded rather than presenting a silently discontinuous stream.
type logClient struct {
	mu     sync.Mutex
	buf    []logEntry
	closed bool
	gap    bool
	wake   chan struct{}
}

type logSubscription struct {
	snapshot []logEntry
	client   *logClient
	firstID  uint64
	lastID   uint64
	reset    bool
	gap      bool
}

func newLogSession(runIDs ...string) *LogSession {
	runID := ""
	if len(runIDs) != 0 {
		runID = runIDs[0]
	}
	return &LogSession{runID: runID, clients: make(map[*logClient]struct{})}
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

func (c *logClient) push(entry logEntry) {
	c.mu.Lock()
	c.buf = append(c.buf, entry)
	if len(c.buf) > maxLogLines {
		c.buf = c.buf[len(c.buf)-maxLogLines:]
		c.gap = true
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

func (c *logClient) take() (lines []logEntry, closed, gap bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	lines, c.buf = c.buf, nil
	gap, c.gap = c.gap, false
	return lines, c.closed, gap
}

// subscribe atomically takes a retained snapshot and registers for new lines.
// A cursor from another run resets to the full snapshot. A cursor older than
// the retained range reports a gap and likewise returns the full snapshot.
func (s *LogSession) subscribe(cursorRunID string, afterID uint64, hasCursor bool) logSubscription {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := logSubscription{client: &logClient{wake: make(chan struct{}, 1)}}
	if len(s.lines) != 0 {
		result.firstID = s.lines[0].id
		result.lastID = s.lines[len(s.lines)-1].id
	}
	if hasCursor && (cursorRunID != s.runID || afterID > s.nextID) {
		result.reset = true
		afterID = 0
	} else if hasCursor && result.firstID > 0 && afterID < result.firstID-1 {
		result.gap = true
		afterID = 0
	}

	first := len(s.lines)
	for i, entry := range s.lines {
		if entry.id > afterID {
			first = i
			break
		}
	}
	result.snapshot = append([]logEntry(nil), s.lines[first:]...)
	if s.closed {
		result.client.closed = true
		result.client.signal()
	} else {
		s.clients[result.client] = struct{}{}
	}
	log.Printf("log subscriber connected run=%s retained=%d-%d cursor=%s:%d reset=%t gap=%t closed=%t",
		s.runID, result.firstID, result.lastID, cursorRunID, afterID, result.reset, result.gap, s.closed)
	return result
}

func (s *LogSession) unsubscribe(c *logClient) {
	s.mu.Lock()
	delete(s.clients, c)
	first, last := uint64(0), uint64(0)
	if len(s.lines) != 0 {
		first, last = s.lines[0].id, s.lines[len(s.lines)-1].id
	}
	s.mu.Unlock()
	log.Printf("log subscriber disconnected run=%s retained=%d-%d", s.runID, first, last)
}

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
	first, last := uint64(0), uint64(0)
	if len(s.lines) != 0 {
		first, last = s.lines[0].id, s.lines[len(s.lines)-1].id
	}
	log.Printf("log session completed run=%s retained=%d-%d", s.runID, first, last)
}
