package com.apimanager.controller;

import com.apimanager.model.Repo;
import com.apimanager.model.RepoInfo;
import com.apimanager.service.*;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ResponseStatusException;
import org.springframework.web.servlet.mvc.method.annotation.SseEmitter;

import java.io.File;
import java.io.IOException;
import java.util.List;
import java.util.Map;
import java.util.UUID;

@RestController
public class RepoController {

    private final StoreService store;
    private final GitService git;
    private final ProcessManager processManager;
    private final PortService portService;

    public RepoController(StoreService store, GitService git,
                          ProcessManager processManager, PortService portService) {
        this.store = store;
        this.git = git;
        this.processManager = processManager;
        this.portService = portService;
    }

    @GetMapping("/health")
    public String health() {
        return "ok";
    }

    // --- Collection endpoints ---

    @GetMapping("/api/repos")
    public List<RepoInfo> list() {
        return store.list().stream().map(this::enrich).toList();
    }

    @PostMapping("/api/repos")
    public ResponseEntity<?> add(@RequestBody Map<String, String> body) throws IOException {
        String path = body.get("path");
        if (path == null || path.isBlank()) {
            return ResponseEntity.badRequest().body("missing or invalid path");
        }
        if (!new File(path).exists()) {
            return ResponseEntity.badRequest().body("path does not exist on disk");
        }
        Repo repo = new Repo(newId(), path);
        store.add(repo);
        return ResponseEntity.status(HttpStatus.CREATED).body(enrich(repo));
    }

    // --- Per-repo endpoints ---

    @DeleteMapping("/api/repos/{id}")
    public ResponseEntity<?> remove(@PathVariable String id) throws IOException {
        boolean found = store.remove(id);
        if (!found) return ResponseEntity.notFound().build();
        return ResponseEntity.noContent().build();
    }

    @PostMapping("/api/repos/{id}/fetch")
    public ResponseEntity<?> fetch(@PathVariable String id) {
        Repo repo = store.findById(id);
        if (repo == null) return ResponseEntity.notFound().build();
        try {
            git.fetch(repo.getPath());
            return ResponseEntity.noContent().build();
        } catch (Exception e) {
            return ResponseEntity.internalServerError().body(e.getMessage());
        }
    }

    @GetMapping("/api/repos/{id}/branches")
    public ResponseEntity<?> branches(@PathVariable String id) {
        Repo repo = store.findById(id);
        if (repo == null) return ResponseEntity.notFound().build();
        try {
            return ResponseEntity.ok(git.listBranches(repo.getPath()));
        } catch (Exception e) {
            return ResponseEntity.internalServerError().body(e.getMessage());
        }
    }

    @PostMapping("/api/repos/{id}/checkout")
    public ResponseEntity<?> checkout(@PathVariable String id,
                                      @RequestBody Map<String, String> body) {
        Repo repo = store.findById(id);
        if (repo == null) return ResponseEntity.notFound().build();
        String branch = body.get("branch");
        if (branch == null || branch.isBlank()) {
            return ResponseEntity.badRequest().body("missing branch");
        }
        try {
            git.checkout(repo.getPath(), branch);
            return ResponseEntity.noContent().build();
        } catch (GitService.DirtyWorkingTreeException e) {
            return ResponseEntity.status(HttpStatus.CONFLICT)
                    .body(Map.of("error", "dirty", "files", e.getFiles()));
        } catch (Exception e) {
            return ResponseEntity.internalServerError().body(e.getMessage());
        }
    }

    @PostMapping("/api/repos/{id}/start")
    public ResponseEntity<?> start(@PathVariable String id) {
        Repo repo = store.findById(id);
        if (repo == null) return ResponseEntity.notFound().build();
        try {
            processManager.start(id, repo.getPath());
            return ResponseEntity.noContent().build();
        } catch (IllegalStateException e) {
            return ResponseEntity.status(HttpStatus.CONFLICT).body(e.getMessage());
        } catch (IOException e) {
            return ResponseEntity.internalServerError().body(e.getMessage());
        }
    }

    @PostMapping("/api/repos/{id}/stop")
    public ResponseEntity<?> stop(@PathVariable String id) {
        if (store.findById(id) == null) return ResponseEntity.notFound().build();
        try {
            processManager.stop(id);
            return ResponseEntity.noContent().build();
        } catch (IllegalStateException e) {
            return ResponseEntity.status(HttpStatus.CONFLICT).body(e.getMessage());
        }
    }

    // Must return SseEmitter directly — ResponseEntity<?> prevents Spring's
    // ResponseBodyEmitterReturnValueHandler from recognising the return type.
    @GetMapping(value = "/api/repos/{id}/logs", produces = MediaType.TEXT_EVENT_STREAM_VALUE)
    public SseEmitter logs(@PathVariable String id) {
        if (store.findById(id) == null)
            throw new ResponseStatusException(HttpStatus.NOT_FOUND, "repo not found");
        LogSession session = processManager.getSession(id);
        if (session == null)
            throw new ResponseStatusException(HttpStatus.NOT_FOUND, "no active session");

        SseEmitter emitter = new SseEmitter(Long.MAX_VALUE);
        LogSession.SubscribeResult sub = session.subscribe();

        emitter.onCompletion(() -> session.unsubscribe(sub.queue()));
        emitter.onError(e -> session.unsubscribe(sub.queue()));

        Thread.ofVirtual().start(() -> {
            try {
                for (String line : sub.snapshot()) {
                    emitter.send(SseEmitter.event().data(line));
                }
                while (true) {
                    String line = sub.queue().take();
                    if (line.equals(LogSession.POISON)) break;
                    emitter.send(SseEmitter.event().data(line));
                }
                emitter.complete();
            } catch (Exception e) {
                session.unsubscribe(sub.queue());
                try { emitter.completeWithError(e); } catch (Exception ignored) {}
            }
        });

        return emitter;
    }

    // --- Helpers ---

    private RepoInfo enrich(Repo repo) {
        RepoInfo info = new RepoInfo();
        info.setId(repo.getId());
        info.setPath(repo.getPath());
        info.setStatus(processManager.isRunning(repo.getId()) ? "running" : "stopped");
        if (!new File(repo.getPath()).exists()) {
            info.setPathError(true);
            return info;
        }
        try {
            info.setCurrentBranch(git.currentBranch(repo.getPath()));
        } catch (Exception ignored) {}
        info.setPort(portService.readPort(repo.getPath()));
        return info;
    }

    private String newId() {
        return UUID.randomUUID().toString().replace("-", "").substring(0, 16);
    }
}
