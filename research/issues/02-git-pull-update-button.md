# Git pull (update) button per repo

## What to build

Add an "Update" button on each repo card that runs `git pull --ff-only` on the currently checked-out branch. Before pulling, the app pre-checks for dirty files using the existing dirty-state detection logic and blocks the operation with a clear message if uncommitted changes are present. If the pull itself fails (e.g. diverged branch), the raw git error is surfaced in the UI.

## Acceptance criteria

- [ ] Each repo card has an "Update" button placed next to the branch selector
- [ ] Clicking Update while the working tree is dirty shows a blocking error (no pull attempted)
- [ ] A successful `git pull --ff-only` updates the branch and gives visual confirmation
- [ ] A failed pull (diverged branch or other git error) displays the raw git error message
- [ ] Button is disabled / shows a spinner while the pull is in progress

## Blocked by

None - can start immediately
