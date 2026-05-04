package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Reads server.port from application.yml next to pom.xml.
// Returns 0 if the file is absent, unreadable, or the key is not set.
func readPort(repoPath string) int {
	data, err := os.ReadFile(filepath.Join(repoPath, "application.yml"))
	if err != nil {
		return 0
	}
	inServer := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "server:" {
			inServer = true
			continue
		}
		if inServer {
			if strings.HasPrefix(trimmed, "port:") {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, "port:"))
				port, err := strconv.Atoi(val)
				if err != nil {
					return 0
				}
				return port
			}
			// Left a top-level key — no longer inside server block
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && !strings.HasPrefix(trimmed, "#") {
				inServer = false
			}
		}
	}
	return 0
}
