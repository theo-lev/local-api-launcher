package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

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
	store, err := NewStore("config.json")
	if err != nil {
		log.Fatal("failed to load config:", err)
	}

	h := &repoHandlers{store: store, manager: NewProcessManager()}

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
		log.Fatal("failed to load embedded frontend:", err)
	}
	mux.Handle("/", spaHandler(frontendFS))

	log.Println("API Manager running at http://localhost:8080")
	if err := http.ListenAndServe(":8080", corsMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}
