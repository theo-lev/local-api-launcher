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

Then open **http://localhost:8080** in your browser.

No installation required. The binary is fully self-contained — no Node.js, no Go, no admin rights needed.

A `config.json` file is created next to the binary on first run to persist your registered repos and settings.

## Features

- **Register repos** — add any local path that contains a `pom.xml`
- **Branch management** — fetch remote branches and switch with one click; blocked on uncommitted changes with a list of dirty files shown
- **Start / Stop** — runs `mvn spring-boot:run -DskipTests` in the repo directory
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
