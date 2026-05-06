# 01 — Project Scaffold

## What to build

Set up the full project skeleton so that both the Go backend and the React frontend start with a single command. No business logic yet — just the wiring that lets everything talk to each other.

- `start.sh` launches the Go backend on `:8080` and the React dev server on `:3000`
- Go module initialised with a minimal HTTP server (one health-check route is enough)
- React app initialised (Vite or CRA) with a placeholder page
- CORS configured on the Go side so the React dev server can hit the API

## Acceptance criteria

- [ ] Running `./start.sh` starts both processes with a single command
- [ ] `GET http://localhost:8080/health` returns 200
- [ ] The React app loads at `http://localhost:3000` without errors
- [ ] The React app can successfully fetch from the Go backend (CORS works)
- [ ] Stopping `start.sh` (Ctrl-C) terminates both processes

## Blocked by

None — can start immediately.
