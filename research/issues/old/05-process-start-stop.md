# 05 — Process Start and Stop

## What to build

Allow the user to start and stop a Maven process per repo. Multiple repos can run simultaneously. The UI reflects the live running state.

End-to-end: user clicks Start → `mvn spring-boot:run -DskipTests` spawns in the repo directory → status dot turns green → user clicks Stop → process receives SIGTERM → status dot returns to grey.

## Acceptance criteria

- [ ] `POST /api/repos/:id/start` spawns `mvn spring-boot:run -DskipTests` with the repo root as working directory
- [ ] `POST /api/repos/:id/stop` sends SIGTERM to the running process
- [ ] Starting an already-running repo returns an error (no duplicate processes)
- [ ] Multiple repos can be started and run simultaneously
- [ ] Repo status transitions to `running` on start and back to `stopped` on exit (crash or clean)
- [ ] The UI Start button becomes a Stop button while the process is running
- [ ] The status dot in the left panel updates to reflect the current state
- [ ] Stopping a repo that is not running returns a clear error

## Blocked by

#03 — Repo list with branch and port
