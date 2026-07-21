package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogSessionReplayIsRunScoped(t *testing.T) {
	s := newLogSession("run-b")
	for _, line := range []string{"one", "two", "three"} {
		s.append(line)
	}

	same := s.subscribe("run-b", 1, true)
	if same.reset || same.gap || len(same.snapshot) != 2 || same.snapshot[0].line != "two" {
		t.Fatalf("same-run replay = %+v, want entries after cursor without reset", same)
	}
	s.unsubscribe(same.client)

	other := s.subscribe("run-a", 3, true)
	if !other.reset || other.gap || len(other.snapshot) != 3 {
		t.Fatalf("other-run replay = %+v, want reset and full snapshot", other)
	}
	s.unsubscribe(other.client)
}

func TestLogSessionReportsRetentionGap(t *testing.T) {
	s := newLogSession("run-a")
	for i := 0; i < maxLogLines+5; i++ {
		s.append("line")
	}
	sub := s.subscribe("run-a", 1, true)
	defer s.unsubscribe(sub.client)
	if !sub.gap || sub.reset {
		t.Fatalf("gap=%t reset=%t, want explicit retention gap", sub.gap, sub.reset)
	}
	if len(sub.snapshot) != maxLogLines || sub.snapshot[0].id != 6 {
		t.Fatalf("snapshot range = %d-%d (%d entries)", sub.snapshot[0].id, sub.snapshot[len(sub.snapshot)-1].id, len(sub.snapshot))
	}
}

func TestPipeLinesKeepsFinalLongAndEmbeddedCarriageReturn(t *testing.T) {
	s := newLogSession("run")
	long := strings.Repeat("x", 128*1024)
	pipeLines(strings.NewReader("first\r\ninside\rline\n"+long), s)
	if len(s.lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(s.lines))
	}
	if s.lines[0].line != "first" || s.lines[1].line != "inside\rline" || s.lines[2].line != long {
		t.Fatal("line contents were altered")
	}
}

func TestPipeLinesDecodesWindows1252WithoutChangingUTF8(t *testing.T) {
	s := newLogSession("run")
	windows1252 := []byte("T\xe2che Synchronization non lanc\xe9e \x96 \x80\n")
	validUTF8 := "Déjà prêt — 日本語\n"
	pipeLines(bytes.NewReader(append(windows1252, []byte(validUTF8)...)), s)
	if len(s.lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(s.lines))
	}
	if got, want := s.lines[0].line, "Tâche Synchronization non lancée – €"; got != want {
		t.Fatalf("Windows-1252 line = %q, want %q", got, want)
	}
	if got, want := s.lines[1].line, strings.TrimSuffix(validUTF8, "\n"); got != want {
		t.Fatalf("UTF-8 line = %q, want %q", got, want)
	}
}

func TestSSELogEntryUsesRunCursorAndJSONEncoding(t *testing.T) {
	var out bytes.Buffer
	line := "before\rafter\nembedded"
	if err := writeLogEntry(&out, "run-7", logEntry{id: 42, line: line}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.HasPrefix(text, "id: run-7:42\n") || strings.Contains(text, "before\rafter") {
		t.Fatalf("unsafe or missing SSE cursor: %q", text)
	}
	data := strings.TrimSuffix(strings.SplitN(text, "data: ", 2)[1], "\n\n")
	var payload struct {
		RunID string `json:"runId"`
		Line  string `json:"line"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.RunID != "run-7" || payload.Line != line {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestLogsHandlerResetsCursorFromAnotherRun(t *testing.T) {
	store, err := NewStore(t.TempDir() + "/config.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Add(Repo{ID: "repo", Path: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	manager := NewProcessManager(store)
	s := newLogSession("run-new")
	s.append("retained\routput")
	s.closeAll()
	manager.sessions["repo"] = s
	h := &repoHandlers{store: store, manager: manager}

	req := httptest.NewRequest("GET", "/api/repos/repo/logs?after=run-old%3A99", nil)
	rec := httptest.NewRecorder()
	h.logs(rec, req, "repo")
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, "event: session-reset") ||
		!strings.Contains(body, "id: run-new:1") || !strings.Contains(body, "event: session-end") {
		t.Fatalf("unexpected SSE response (%d): %s", rec.Code, body)
	}
}
