package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed all:dist
var distFS embed.FS

// fatal logs the error, mirrors it to a file (a double-clicked exe's console
// vanishes on exit, taking the message with it), then pauses on Windows.
func fatal(v ...any) {
	msg := fmt.Sprintln(v...)
	log.Print(msg)
	os.WriteFile("api-manager-error.log", []byte(time.Now().Format(time.RFC3339)+" "+msg), 0644)
	pauseOnExit()
	os.Exit(1)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func spaHandler(assets fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if f, err := assets.Open(path); err == nil {
				f.Close()
			} else {
				http.ServeFileFS(w, r, assets, "index.html")
				return
			}
		}
		fileServer.ServeHTTP(w, r)
	})
}

func main() {
	store, err := NewStore(configPath())
	if err != nil {
		fatal("failed to load config:", err)
	}

	manager := NewProcessManager(store)
	for id, info := range store.GetPids() {
		if isAlive(info.Pid) {
			manager.Reconnect(id, info.Pid, info.EnvID, info.EnvName)
		} else {
			store.RemovePid(id)
		}
	}
	h := &repoHandlers{store: store, manager: manager}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.getSettings(w, r)
		case http.MethodPut:
			h.putSettings(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/environments", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.listEnvs(w, r)
		case http.MethodPost:
			h.createEnv(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/environments/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/environments/")
		switch {
		case r.Method == http.MethodPut && id == "active":
			h.setActiveEnv(w, r)
		case r.Method == http.MethodPut && id != "":
			h.updateEnv(w, r, id)
		case r.Method == http.MethodDelete && id != "":
			h.deleteEnv(w, r, id)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})

	mux.HandleFunc("/api/repos", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.list(w, r)
		case http.MethodPost:
			h.add(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/repos/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/api/repos/")
		parts := strings.SplitN(rest, "/", 2)
		id := parts[0]
		sub := ""
		if len(parts) > 1 {
			sub = parts[1]
		}
		switch {
		case r.Method == http.MethodDelete && sub == "":
			h.remove(w, r, id)
		case r.Method == http.MethodPost && sub == "fetch":
			h.fetch(w, r, id)
		case r.Method == http.MethodPost && sub == "pull":
			h.pull(w, r, id)
		case r.Method == http.MethodGet && sub == "branches":
			h.branches(w, r, id)
		case r.Method == http.MethodPost && sub == "checkout":
			h.checkout(w, r, id)
		case r.Method == http.MethodPost && sub == "start":
			h.start(w, r, id)
		case r.Method == http.MethodPost && sub == "stop":
			h.stop(w, r, id)
		case r.Method == http.MethodGet && sub == "logs":
			h.logs(w, r, id)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})

	frontendFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		fatal("failed to load embedded frontend:", err)
	}
	mux.Handle("/", spaHandler(frontendFS))

	log.Println("API Manager running at http://localhost:8080")
	if err := http.ListenAndServe(":8080", corsMiddleware(mux)); err != nil {
		fatal(err, "(is port 8080 already in use by another program?)")
	}
}
