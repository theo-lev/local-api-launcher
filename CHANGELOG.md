# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.3] - 2026-06-09

### Fixed

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

- First backend tests (`backend/config_test.go`), covering config
  save/load round-trips and recovery from a truncated config file.

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
[0.0.3]: https://github.com/theo-lev/local-api-launcher/compare/0.0.2...0.0.3
[0.0.2]: https://github.com/theo-lev/local-api-launcher/compare/0.0.1...0.0.2
[0.0.1]: https://github.com/theo-lev/local-api-launcher/releases/tag/0.0.1
