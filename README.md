# API Manager

A lightweight local dashboard for managing multiple Spring Boot Maven APIs. Register repo paths, switch branches safely, start/stop services, and stream live logs — all from one browser tab.

## Usage

Download the binary for your platform from the [releases](../../releases) page, then run it:

| Platform | Binary |
|---|---|
| macOS (Intel) | `api-manager-darwin-amd64` |
| macOS (Apple Silicon) | `api-manager-darwin-arm64` |
| Linux (x64) | `api-manager-linux-amd64` |
| Linux (ARM64) | `api-manager-linux-arm64` |
| Windows (x64) | `api-manager-windows-amd64.exe` |

**macOS / Linux:**
```sh
chmod +x api-manager-darwin-arm64   # or the variant for your machine
./api-manager-darwin-arm64
```

**Windows:**
```
api-manager-windows-amd64.exe
```

Then open the URL printed by the executable: **http://127.0.0.1:9000** by
default. If port 9000 is occupied, API Manager automatically tries ports
9001–9010 and prints the selected URL.

To select an exact port, use either the command-line flag or environment
variable (the flag takes precedence):

```sh
./api-manager-linux-amd64 --port 9200
API_MANAGER_PORT=9200 ./api-manager-linux-amd64
```

On Windows:

```bat
api-manager-windows-amd64.exe --port 9200
set API_MANAGER_PORT=9200
api-manager-windows-amd64.exe
```

Explicit ports never fall back silently. If the selected port is occupied, the
executable exits with an actionable error. Run `api-manager --help` to see the
startup option.

No installation required. The binary is fully self-contained — no Node.js, no Go, no admin rights needed.

A `config.json` file is created next to the binary on first run to persist your registered repos and settings.

## Features

- **Register repos** — add any local path that contains a `pom.xml`
- **Branch management** — fetch remote branches and switch with one click; blocked on uncommitted changes with a list of dirty files shown
- **Start / Stop** — runs `mvn spring-boot:run -DskipTests` in the repo directory
- **Environments** — define named sets of `KEY=VALUE` variables, pick the active one, and switch sets when you change context; the active set is injected into every API you start, and each running API shows which environment it was started under
- **Live logs** — stdout and stderr streamed in real time via SSE; up to 2000 lines buffered
- **Port display** — reads `server.port` from `application.yml` automatically
- **Maven path** — configure a custom `mvn` executable path in Settings (⚙) if Maven is not on your `PATH`
- **Multi-service** — run multiple APIs simultaneously

## Building from source

Requires [Go 1.21+](https://go.dev) and [Node.js 18+](https://nodejs.org).

**macOS / Linux:**
```sh
./build.sh
```

**Windows:**
```
build.bat
```

Both scripts build the React frontend, embed it into the Go binary, and cross-compile for all platforms. Output goes to `dist/`.

For frontend development, the Vite proxy targets
`http://127.0.0.1:9000`. Set `API_MANAGER_DEV_TARGET` to proxy to a backend on a
different port, for example `API_MANAGER_DEV_TARGET=http://127.0.0.1:9200`.

## Troubleshooting startup

- If all automatic ports from 9000 through 9010 are occupied, stop one of the
  conflicting programs or start API Manager with an available `--port`.
- An explicitly configured port is deterministic and will not use the automatic
  fallback range.
- API Manager listens only on `127.0.0.1`; it is intentionally unavailable from
  other devices on the network.
