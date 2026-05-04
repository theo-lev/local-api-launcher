# API Manager — Research & Spec

## Problem

Multiple Spring Boot Maven APIs live in separate git repos. The dev workflow requires:
- Switching branches per API without leaving a central interface
- Launching and stopping APIs on demand
- Watching real-time logs while they run

There is no existing local tool that ties git branch management + process lifecycle + log streaming into one UI for a multi-repo Spring Boot setup.

---

## Stack Decisions

| Layer | Choice | Reason |
|---|---|---|
| Frontend | React | Standard, fast to build a reactive UI |
| Backend | Go | Single binary, easy process management, good SSE/HTTP support |
| Distribution | `start.sh` (WSL2) | Team works on Windows + WSL2, one script starts both |
| Config | `config.json` next to the Go binary | Simple, no install path assumptions |

---

## Architecture

```
start.sh
  ├── go run ./backend     → :8080 (API + SSE log streams)
  └── npm run dev          → :3000 (React UI)
```

The Go backend:
- Manages a map of running processes (one per repo, multiple can run simultaneously)
- Streams stdout/stderr from each Maven process via **SSE** (unidirectional, simpler than WebSocket)
- Runs git commands (`fetch`, `status`, `checkout`, `branch`) as subprocesses
- Reads `application.yml` next to `pom.xml` to extract the configured port

---

## UI Layout

```
┌─────────────────┬──────────────────────────────────────┐
│  Repo List      │  Detail Panel                        │
│                 │                                      │
│  ● api-users    │  [Branch: main ▼]  [Fetch]  [Start] │
│    main         │  Port: 8081                          │
│                 │                                      │
│  ○ api-orders   ├──────────────────────────────────────┤
│    feature/x    │  Logs                                │
│                 │  > Started on port 8081              │
│  ○ api-billing  │  > Connected to DB                   │
│    main         │  > ...                               │
└─────────────────┴──────────────────────────────────────┘
```

**Left panel per repo:** name + current branch + status dot (green = running, grey = stopped)

**Detail panel top:** branch selector, fetch button, start/stop button, port (read-only)

**Detail panel bottom:** live log stream, cleared on each new start

---

## Key Behaviours

### Repo Registration
- Manual: user adds each repo path explicitly
- Stored in `config.json` next to the binary
- Auto-discovery (scan for `pom.xml`) deferred to a later version

### Branch Switching
1. User clicks fetch → `git fetch` runs for that repo
2. Branch list populates from local branches (post-fetch)
3. User selects a branch → app runs `git status`
4. If uncommitted changes exist: **block and display dirty files** — never auto-stash
5. If clean: run `git checkout <branch>`

### Launching an API
- Command: `mvn spring-boot:run -DskipTests`
- Working directory: the repo root
- Multiple APIs can run simultaneously
- Port is read from `application.yml` (next to `pom.xml`), not overridable in the UI

### Log Streaming
- Go captures stdout/stderr from the Maven process
- Streamed to the frontend via SSE (`text/event-stream`)
- Logs are held in memory only — cleared when the process stops
- No persistence, no history across restarts

### Process Lifecycle
- Start → spawns `mvn` subprocess, opens SSE stream
- Stop → sends SIGTERM to the process
- Crash or clean exit → status returns to "stopped" (no distinction between crash and clean stop)

---

## What We Ruled Out

| Idea | Why dropped |
|---|---|
| Auto-fetch on app load | Adds latency, can fail silently (SSH key, network) |
| Port override in UI | Each API owns its port via `application.yml` |
| Log history across restarts | Overkill for a dev tool — current session only |
| Auth | Localhost-only, no real attack surface |
| Electron/Tauri | Overkill, a local web app is sufficient |
| Separate Windows `.bat` script | Team standardised on WSL2 |
| WebSocket for logs | SSE is enough — logs are server→client only |
| Auto-stash on branch switch | Too magic, can cause confusion |
| Crash vs clean stop distinction | Not needed — user reads the logs |

---

## Open Questions (deferred)

- Auto-discovery of repos by scanning a base directory for `pom.xml`
- Manual fetch button vs auto-fetch on repo selection
- `mvn package` + `java -jar` launch mode for closer-to-prod runs
- Search/filter in the log panel
