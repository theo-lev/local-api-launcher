# 02 — Repo Registration

## What to build

Allow the user to register and remove local repo paths. Registered repos persist across tool restarts via `config.json` stored next to the Go binary.

End-to-end: user enters a path in the UI → it appears in the repo list → survives a tool restart → user can remove it.

## Acceptance criteria

- [ ] `POST /api/repos` with a local path adds the repo and writes `config.json`
- [ ] `DELETE /api/repos/:id` removes the repo and updates `config.json`
- [ ] `GET /api/repos` returns the current list of registered repos
- [ ] Restarting the backend reloads repos from `config.json` (no data loss)
- [ ] The UI has an input to add a repo by path
- [ ] The UI shows a remove control per repo
- [ ] Submitting a path that does not exist on disk returns a clear error

## Blocked by

#01 — Project scaffold
