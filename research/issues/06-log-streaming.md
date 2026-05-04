# 06 — Log Streaming

## What to build

Stream stdout and stderr from each running Maven process to the frontend via SSE. Logs are held in memory only and cleared each time the process starts.

End-to-end: user starts a repo → log panel populates in real time with Maven output → user stops the repo → next start clears the panel and streams fresh output.

## Acceptance criteria

- [ ] `GET /api/repos/:id/logs` returns an SSE stream (`text/event-stream`) of log lines for the running process
- [ ] Both stdout and stderr from the Maven process are captured and forwarded
- [ ] A client connecting after the process has already started receives the buffered lines from the current session, then continues to receive new lines live
- [ ] The log buffer is cleared when the process starts (not when it stops)
- [ ] The log panel in the UI connects to the SSE endpoint and appends lines in real time
- [ ] The log panel is cleared visually each time the user starts the process
- [ ] Closing the SSE connection (e.g. navigating away) does not crash the backend or kill the process
- [ ] When the process stops, the SSE stream closes cleanly

## Blocked by

#05 — Process start and stop
