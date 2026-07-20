# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.6] - 2026-07-20

### Added

- **Configurable manager port.** API Manager now listens on
  `http://127.0.0.1:9000` by default. The port can be selected with
  `--port <number>` or `API_MANAGER_PORT`, with the command-line flag taking
  precedence. Explicit ports fail with a clear error when unavailable.
- When the default port is occupied, API Manager automatically selects the
  first available port from 9001 through 9010 and prints the exact URL used.
- The Vite development proxy can target another backend through
  `API_MANAGER_DEV_TARGET`.
- Removing an API from the manager now requires confirmation and explains that
  the repository files will not be deleted.

### Changed

- **Redesigned API and log workspace.** APIs are presented as larger,
  information-rich rows with their status, environment, branch, port, path,
  repository actions, and a dedicated far-right Logs button.
- The API list occupies 40% of the window by default and can be resized with a
  draggable divider. Logs use the remaining pane, and the layout stacks on
  narrower screens.
- The Environments dialog is larger and can be resized on desktop for easier
  editing of environment-variable sets.
- Startup now binds only to the local loopback interface instead of exposing the
  unauthenticated manager on every network interface.

### Fixed

- **Live logs remain current after interruptions.** Every SSE log entry now has
  a sequence ID, and reconnecting clients resume from their last received entry
  without duplicating the retained history or missing subsequent output.
- Log streams now send periodic keep-alive events, disable proxy buffering, and
  stop cleanly when a client write fails.
- API output lines larger than 1 MiB no longer terminate log capture. Final
  partial lines and buffered stdout/stderr are flushed before a process session
  closes.
- The browser log buffer remains bounded while preserving the latest retained
  output, preventing long-running sessions from growing memory indefinitely.

## [0.0.5] - 2026-06-19

### Fixed

- **Live logs no longer stop updating during bursts of output.** Each log
  viewer (SSE subscriber) had a small 256-slot buffered channel written with a
  non-blocking send, so once the browser couldn't drain it fast enough the
  buffer filled and every further line was silently dropped — the stream stayed
  open but appeared frozen. Subscribers now get a per-client queue that is
  drained in batches and bounded to the same 2000-line retention as the session
  itself, so a slow reader can fall behind and catch up without losing lines,
  while the producer (and through it the child process) is never blocked. The
  final lines emitted as a process exits are also flushed before the stream
  closes, instead of being lost when the session ended.

## [0.0.4] - 2026-06-19

### Added

- **Environments** — define named sets of environment variables and pick which
  one is active. The active set's variables are injected into every API you
  start, on top of your inherited shell environment (the set overrides it), with
  `JAVA_HOME` from the JDK path still winning last. Switch the active set from
  the sidebar; manage sets (create/rename/edit/delete) in a dedicated editor.
  Variables are authored in dotenv style (`KEY=VALUE` per line, `#` comments).
  Switching is non-destructive: running APIs keep the environment they were
  started under, shown as a badge, and pick up a new set only on restart. The
  launched environment is persisted, so the badge survives an app restart and
  reconnect. `config.json` gains `envSets` / `activeEnvId`, and the `pids` map
  now records the launch environment (old `pids` files are migrated on load).

## [0.0.3] - 2026-06-10

### Fixed

- The first file in the "uncommitted changes" list no longer has its first
  letter cut off (`pom.xml` was shown as `om.xml`): the leading status
  character of the first `git status --porcelain` line was being trimmed
  before parsing.
- **Windows: stopping an API now kills the whole process tree** ([#3]).
  `mvn.cmd` is a cmd.exe wrapper that spawns Java as a child process; the stop
  button only killed the wrapper, leaving the Spring Boot API running in the
  background. Stop now uses `taskkill /T /F` to terminate the wrapper and all
  of its descendants, including for processes reconnected after an app restart.
- **`config.json` can no longer be wiped by a crash.** Saves previously
  truncated the file before writing, so a process killed mid-save (e.g. by
  antivirus) left a blank config and all registered repos were lost. Saves are
  now atomic (write to temp file, then rename), and the previous good version
  is kept as `config.json.bak`. If the config is ever blank or corrupt at
  startup, it is restored from the backup automatically.

### Changed

- Clicking Start/Stop now gives immediate feedback: the button shows
  "Starting…" / "Stopping…" with a spinner and stays that way until the new
  status is actually displayed. Fetch, Update, and branch switching got the
  same treatment, and disabled/pressed buttons now have visual states.
- On Windows, started APIs no longer pop up a separate command prompt window
  (`CREATE_NO_WINDOW`); their output was already captured in the log view.
- Fatal startup errors (e.g. port 8080 already in use) are now written to
  `api-manager-error.log` next to the executable, and on Windows the console
  waits for a key press instead of closing instantly, so the error can
  actually be read.
- When no `config.json` exists in the working directory, it is now created
  next to the executable instead of in the working directory, so launching
  from another folder no longer starts with an empty config. Existing configs
  in the working directory keep working as before.

### Added

- First backend tests (`backend/config_test.go`, `backend/git_test.go`),
  covering config save/load round-trips, recovery from a truncated config
  file, and dirty-file parsing.

### Security

- Frontend dependencies are pinned to exact versions (no more `^` ranges),
  so installs can't silently pull newer, unvetted releases.
- Updated the transitive `brace-expansion` dependency to resolve a moderate
  ReDoS advisory ([GHSA-jxxr-4gwj-5jf2]); `npm audit` is now clean.

[GHSA-jxxr-4gwj-5jf2]: https://github.com/advisories/GHSA-jxxr-4gwj-5jf2

## [0.0.2] - 2026-05-06

### Added

- Global JDK path setting (`JAVA_HOME` passed to started APIs).
- Git pull button to update a repo's current branch.
- Process reconnection: started API PIDs are persisted, so the manager finds
  and re-attaches to still-running APIs after the app was closed.

## [0.0.1] - 2026-05-05

### Added

- Initial release: Go backend with embedded React frontend, served at
  `http://localhost:8080`.
- Register local Spring Boot repos and list them with current branch,
  configured port, and running status.
- Start and stop APIs via `mvn spring-boot:run` with a configurable
  Maven path.
- Git integration: fetch, branch listing, and branch switching with
  dirty-working-tree detection.
- Live log streaming per API via server-sent events.
- Cross-platform builds (macOS, Linux, Windows) via `build.sh` / `build.bat`.

[#3]: https://github.com/theo-lev/local-api-launcher/issues/3
[Unreleased]: https://github.com/theo-lev/local-api-launcher/compare/0.0.6...HEAD
[0.0.6]: https://github.com/theo-lev/local-api-launcher/compare/0.0.5...0.0.6
[0.0.5]: https://github.com/theo-lev/local-api-launcher/compare/0.0.4...0.0.5
[0.0.4]: https://github.com/theo-lev/local-api-launcher/compare/0.0.3...0.0.4
[0.0.3]: https://github.com/theo-lev/local-api-launcher/compare/0.0.2...0.0.3
[0.0.2]: https://github.com/theo-lev/local-api-launcher/compare/0.0.1...0.0.2
[0.0.1]: https://github.com/theo-lev/local-api-launcher/releases/tag/0.0.1
