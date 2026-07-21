# 07 — Fix Log Session Lifecycle and Replay

## Problem

Live logs can stop after an arbitrary event even though the launched API keeps
producing output. Logs can also be unavailable after switching to another API
and back.

There is no 49-line SSE limit. In the reported example, the visible stream and
the supposedly subsequent output came from different JVM launches (PIDs 35792
and 20336). SSE sequence numbers currently restart at 1 for every launch.

## Root cause

Process cleanup is keyed only by repository ID, not by the individual launch.
`Stop` removes the current process from `ProcessManager.procs` immediately,
which allows another launch to start. When the old process's asynchronous
waiter later finishes, it unconditionally deletes `procs[id]` and the persisted
PID. If a new launch is already stored under that ID, the old waiter therefore
removes the new launch's state.

The repository is then reported as stopped. The frontend closes its
`EventSource` whenever the status is not `running`, so the new API's remaining
output is no longer delivered even if the Java process is still alive.

Switching views amplifies the problem: the log viewer is remounted and loses
its local buffer, but it refuses to request the backend's retained log session
when the repository is reported as stopped. This also prevents retrieving the
final retained logs of a process that exited normally.

Related weaknesses:

- Windows process termination ignores `taskkill` failures and can report a
  successful stop while the process remains alive.
- SSE cursors contain only a sequence number and cannot identify the launch to
  which they belong.
- A retention gap is silent when a client falls behind the 2,000-line limit.
- Log lines are written directly into SSE frames without explicit encoding for
  carriage returns or other unusual output.
- There are no automated tests for log sessions, SSE replay, view switching,
  or rapid stop/restart lifecycle races.

## What to build

Make process state and log replay launch-scoped so an older process can never
modify a newer launch. Allow the latest retained session to be viewed
independently of whether the process is currently running.

### Backend

- Give every launch a unique run/session ID and store it on both
  `managedProcess` and `LogSession`.
- Make the process waiter remove process state and persisted PID only when the
  registered process still has the same run ID (or is the same process
  instance).
- Keep a process in a `stopping` state until termination and pipe draining are
  complete. Reject or serialize a restart while the old launch is still being
  reaped.
- Return and handle errors from Windows and Unix process termination.
- Scope SSE cursors to a launch, for example `runID:sequence`.
- When a cursor belongs to another launch, explicitly reset the client with the
  current session snapshot rather than silently applying the numeric sequence.
- Detect when requested entries have left the 2,000-line retention window and
  emit an explicit gap/reset event.
- Encode SSE data safely, including output containing carriage returns.
- Keep the latest completed session available from `/logs` until the next
  launch replaces it.

### Frontend

- Do not make retained-log retrieval depend solely on
  `repo.status === "running"`.
- Preserve log data and cursors per repository and run ID when switching views,
  or reliably restore them from the backend snapshot.
- Clear a repository's displayed logs only after the backend identifies a new
  run, rather than relying only on the Start button handler.
- Handle explicit session-reset and retention-gap events.
- Flush pending received lines during viewer cleanup so view changes do not
  discard the final UI batch.
- Use one well-defined reconnect strategy and resume only within the matching
  run ID.

### Observability

- Add diagnostic logging for repository ID, run ID, PID, retained sequence
  range, subscriber connect/disconnect, and process cleanup decisions.

## Acceptance criteria

- [ ] An old process waiter cannot remove or mark a newer launch as stopped.
- [ ] Rapid Stop → Start cannot leave a live API untracked by the manager.
- [ ] A failed process-tree termination is reported and does not silently clear
      process state.
- [ ] Every log entry has a run-scoped cursor.
- [ ] Reconnecting within the same run produces no duplicate or missing entries.
- [ ] A cursor from another run produces an explicit reset and the current
      retained snapshot.
- [ ] Falling behind retention produces an explicit gap indication rather than
      a silent discontinuity.
- [ ] Switching repeatedly between running APIs restores each API's retained
      logs and continues live streaming.
- [ ] Switching back to a normally stopped API shows its final retained logs.
- [ ] Starting a new run clears the previous run's display and cannot mix lines
      from different PIDs.
- [ ] Final partial lines, long lines, multiline output, stdout, and stderr are
      delivered correctly.
- [ ] The browser and backend remain bounded to the configured retention limit.

## Test plan

- Add a deterministic process-manager test where launch A is stopped, launch B
  starts, and A's waiter completes after B is registered.
- Add handler tests for initial snapshots, live delivery, reconnect cursors,
  completed-session replay, session resets, and retention gaps.
- Add burst tests exceeding the former 256-entry client queue and the current
  2,000-line retention limit.
- Add tests for long lines, final unterminated lines, embedded carriage returns,
  and interleaved stdout/stderr.
- Add frontend tests for API view switching, stopped-session retrieval,
  reconnects, pending-batch cleanup, and new-run resets.
- Run an end-to-end Windows test using rapid stop/restart and verify that the
  PID, status, run ID, and SSE sequence remain consistent.

## Relevant files

- `backend/process.go`
- `backend/process_windows.go`
- `backend/process_unix.go`
- `backend/logs.go`
- `backend/handlers.go`
- `frontend/src/App.jsx`

## Priority

High — the bug hides application output and can leave a running API detached
from the manager's process state.
