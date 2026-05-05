package com.apimanager.service;

import org.springframework.stereotype.Service;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

@Service
public class PortService {

    /** Reads server.port from application.yml at the repo root. Returns 0 if absent or unparseable. */
    public int readPort(String repoPath) {
        Path yml = Path.of(repoPath, "application.yml");
        if (!Files.exists(yml)) return 0;
        try {
            boolean inServer = false;
            for (String line : Files.readAllLines(yml)) {
                String trimmed = line.strip();
                if (trimmed.equals("server:")) {
                    inServer = true;
                    continue;
                }
                if (inServer) {
                    if (trimmed.startsWith("port:")) {
                        String val = trimmed.substring("port:".length()).strip();
                        try {
                            return Integer.parseInt(val);
                        } catch (NumberFormatException e) {
                            return 0;
                        }
                    }
                    // Left the server block if we hit another top-level key
                    if (!line.isEmpty() && line.charAt(0) != ' ' && line.charAt(0) != '\t'
                            && !trimmed.startsWith("#")) {
                        inServer = false;
                    }
                }
            }
        } catch (IOException ignored) {}
        return 0;
    }
}
