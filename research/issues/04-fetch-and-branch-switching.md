# 04 — Fetch and Branch Switching

## What to build

Let the user fetch remote branches and switch the local branch for a repo, with a safety check that blocks the switch if there are uncommitted changes.

End-to-end: user clicks Fetch → branch list updates → user picks a branch → if dirty, blocked with a file list; if clean, checkout runs and the branch indicator updates.

## Acceptance criteria

- [ ] `POST /api/repos/:id/fetch` runs `git fetch` in the repo directory
- [ ] `GET /api/repos/:id/branches` returns all local branch names
- [ ] `POST /api/repos/:id/checkout` with `{ "branch": "..." }` runs `git checkout` if the working tree is clean
- [ ] If the working tree is dirty, checkout is blocked and the response includes the list of dirty files
- [ ] The UI shows a Fetch button that triggers fetch and refreshes the branch list on completion
- [ ] The UI shows a branch dropdown populated from the branch list
- [ ] Selecting a branch triggers checkout; on success the branch indicator updates
- [ ] On dirty working tree, the UI displays the list of dirty files and does not proceed with checkout
- [ ] Fetch and checkout errors (e.g. network failure, bad ref) are surfaced in the UI

## Blocked by

#03 — Repo list with branch and port
