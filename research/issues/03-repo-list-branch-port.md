# 03 — Repo List with Branch and Port

## What to build

Enrich each registered repo with its current git branch, running status, and configured port. Display this in the left panel and the detail panel header.

End-to-end: user sees each repo with its current branch, a status dot (green/grey), and the port from `application.yml` — without opening a terminal.

## Acceptance criteria

- [ ] `GET /api/repos` response includes `currentBranch`, `status` (running/stopped), and `port` per repo
- [ ] Current branch is read by running `git branch --show-current` in the repo directory
- [ ] Port is parsed from `application.yml` adjacent to `pom.xml`; shows a placeholder if file is absent or port is not set
- [ ] Left panel lists all repos with name, current branch, and a green/grey status dot
- [ ] Clicking a repo opens its detail panel showing branch, port, and status
- [ ] UI handles the case where a repo path no longer exists on disk gracefully (error state, not a crash)

## Blocked by

#02 — Repo registration
