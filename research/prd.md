# PRD: API Manager — Local Multi-Repo Spring Boot Dev Tool

## Problem Statement

Developers working across multiple Spring Boot Maven APIs spread across separate git repositories have no unified local interface to manage their day-to-day dev workflow. Switching branches, starting/stopping services, and watching logs currently requires jumping between multiple terminal windows and git repositories. The context-switching overhead slows down development, especially when testing interactions between several running APIs simultaneously.

## Solution

A lightweight local web application (React frontend + Go backend) that gives developers a single dashboard to manage all their Spring Boot repos: switch branches safely, launch and stop Maven processes, and stream live logs — all from one browser tab. The tool starts with a single shell script and requires no installation.

## User Stories

1. As a developer, I want to register a local repo path so that the app tracks it as a managed API.
2. As a developer, I want to see all registered repos listed in a left panel so that I have an at-a-glance overview of my services.
3. As a developer, I want to see the current git branch for each repo in the list so that I know what each service is running on without opening a terminal.
4. As a developer, I want to see a colored status indicator per repo (green = running, grey = stopped) so that I can tell which services are live at a glance.
5. As a developer, I want to click a repo in the list to open its detail panel so that I can manage it without losing sight of the others.
6. As a developer, I want to see the configured port for a selected repo so that I know which port it will bind to before I start it.
7. As a developer, I want to click a Fetch button for a repo so that remote branches are pulled down without affecting local state.
8. As a developer, I want the branch selector to reflect all locally known branches after a fetch so that I can switch to newly created remote branches.
9. As a developer, I want to select a branch from a dropdown so that I can switch the repo without typing git commands.
10. As a developer, I want the app to check for uncommitted changes before switching branches so that I never lose in-progress work.
11. As a developer, I want the branch switch to be blocked and a list of dirty files shown when there are uncommitted changes so that I'm informed and in control.
12. As a developer, I want the branch checkout to proceed automatically when the working tree is clean so that switching branches is a single click.
13. As a developer, I want a Start button for each repo so that I can launch a service with one click.
14. As a developer, I want the Start button to become a Stop button once the process is running so that I always know the current state and can stop it.
15. As a developer, I want to start multiple repos simultaneously so that I can run interconnected services for end-to-end testing.
16. As a developer, I want the app to run `mvn spring-boot:run -DskipTests` for a repo so that the service starts quickly without running the test suite.
17. As a developer, I want `mvn` to run from the repo's root directory so that it picks up the correct `pom.xml` and working directory configuration.
18. As a developer, I want to see a live log stream for the running process so that I can monitor startup progress and runtime output without a terminal.
19. As a developer, I want the log panel to be cleared when a process starts so that I'm only reading logs from the current session.
20. As a developer, I want stdout and stderr both captured and streamed so that I see error output alongside normal logs.
21. As a developer, I want to click Stop on a running repo so that the process is terminated cleanly.
22. As a developer, I want the repo status to return to "stopped" after the process exits (crash or clean stop) so that the UI accurately reflects reality.
23. As a developer, I want the port to be read from the repo's `application.yml` so that the app reflects the API's own configuration without me having to enter it.
24. As a developer, I want the tool to start with a single `start.sh` command on WSL2 so that there is no complex installation process.
25. As a developer, I want my registered repos to persist between tool restarts so that I don't have to re-register them each session.
26. As a developer, I want the config to be stored in a simple JSON file next to the binary so that I can inspect and edit it manually if needed.

## Implementation Decisions

**Modules**

- **Config store** — reads and writes `config.json` (repo list); simple file-based persistence with a stable load/save interface. No database.
- **Process manager** — spawns, tracks, and terminates Maven subprocesses; one process slot per repo; exposes start/stop/status operations. Holds an in-memory log buffer per process.
- **Log broadcaster** — multiplexes stdout/stderr from a running process to any connected SSE clients for that repo; clears on process start.
- **Git runner** — executes `git fetch`, `git branch`, `git status`, `git checkout` as subprocesses in the repo's working directory; returns structured results (dirty files list, branch list, current branch).
- **Port reader** — parses `application.yml` adjacent to `pom.xml` to extract `server.port`; read-only.
- **HTTP API** — Go HTTP handlers wiring all of the above together; REST for commands, SSE endpoint for log streams.
- **React frontend** — repo list panel + detail panel (branch selector, fetch, start/stop, port display, log viewer); polls or receives SSE for live state.

**API Contracts (HTTP)**

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/repos` | List all repos with current branch, running status, port |
| `POST` | `/api/repos` | Register a new repo path |
| `DELETE` | `/api/repos/:id` | Remove a repo |
| `GET` | `/api/repos/:id/branches` | List local branches |
| `POST` | `/api/repos/:id/fetch` | Run `git fetch` |
| `POST` | `/api/repos/:id/checkout` | Switch branch (`{ "branch": "..." }`); returns error with dirty file list if working tree is dirty |
| `POST` | `/api/repos/:id/start` | Start Maven process |
| `POST` | `/api/repos/:id/stop` | Stop Maven process |
| `GET` | `/api/repos/:id/logs` | SSE stream (`text/event-stream`) |

**Key Architectural Decisions**

- SSE over WebSocket for log streaming — logs are server→client only; SSE is simpler and sufficient.
- In-memory log buffer only — no log persistence across tool restarts.
- No auto-stash — branch switch is blocked on a dirty working tree; the user must resolve it themselves.
- Port is read-only in the UI — each API owns its port via `application.yml`.
- Multiple APIs can run simultaneously — process manager holds a map keyed by repo ID.
- `start.sh` launches both the Go backend (`:8080`) and the React dev server (`:3000`).
- Go binary and `config.json` are co-located; path resolution is relative to the binary, not the working directory.

## Testing Decisions

A good test exercises external behavior through the module's public interface only — no assertions on internal state, no mocking of things the module itself owns.

**Modules to test:**

- **Config store** — write a config with repos, read it back; verify round-trip correctness. Test the "file not found → empty config" path.
- **Git runner** — integration tests against a real temp git repo (created in the test); verify `git status` correctly identifies dirty files; verify `git checkout` is blocked when dirty and succeeds when clean.
- **Port reader** — parse a set of fixture `application.yml` files (port present, port absent, nested YAML); verify returned port or zero-value.
- **Process manager** — spawn a short-lived subprocess (e.g. `echo`), verify status transitions (stopped → running → stopped); verify stop sends SIGTERM and status returns to stopped.
- **HTTP API** — integration tests against a running test server with a temp config and real git repos; cover the start/stop/checkout/fetch endpoints end-to-end.

Log broadcaster and frontend UI are not unit-tested; log output is verified through the HTTP API integration tests via the SSE endpoint.

## Out of Scope

- Auto-discovery of repos by scanning a directory for `pom.xml`
- `mvn package` + `java -jar` launch mode
- Log search/filter in the UI
- Log history across tool restarts
- Auto-fetch on repo selection or app load
- Port override in the UI
- Authentication (localhost-only tool)
- Distinction between crash exits and clean exits
- Windows `.bat` launcher (WSL2 only)
- Electron/Tauri packaging

## Further Notes

- The team runs on Windows + WSL2; all shell scripting should target bash.
- `application.yml` is expected to live next to `pom.xml` in the repo root. If absent or unparseable, the port field in the UI should show a placeholder rather than crash.
- Maven must be available on `$PATH` inside the WSL2 environment where the tool runs.
