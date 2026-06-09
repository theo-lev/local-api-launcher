# PID persistence — survive app restart

## What to build

When a Spring Boot process is started, persist its PID to disk. On app startup, read the stored PIDs, check which processes are still alive, and automatically mark them as running in the UI with a "reconnected" visual indicator (badge or distinct colour). The stop action works by sending a kill signal to the stored PID, same as for freshly started processes.

## Acceptance criteria

- [ ] Process PID is written to disk (e.g. `config.json` or a separate state file) when a process starts
- [ ] On app restart, live PIDs are detected and their repos are shown as running without user action
- [ ] Reconnected processes display a distinct "reconnected" badge/indicator in the UI
- [ ] Stop button works for reconnected processes (kills the PID)
- [ ] Stale PIDs (process no longer alive) are cleaned up silently on startup without showing as running

## Blocked by

None - can start immediately
