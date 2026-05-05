package com.apimanager.service;

import org.springframework.stereotype.Service;

import java.io.BufferedReader;
import java.io.File;
import java.io.IOException;
import java.io.InputStreamReader;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.locks.ReentrantLock;

@Service
public class ProcessManager {

    private final ReentrantLock lock = new ReentrantLock();
    private final Map<String, Process> procs = new HashMap<>();
    private final Map<String, LogSession> sessions = new HashMap<>();

    public void start(String id, String repoPath) throws IOException {
        lock.lock();
        try {
            if (procs.containsKey(id)) throw new IllegalStateException("already running");

            ProcessBuilder pb = new ProcessBuilder("mvn", "spring-boot:run", "-DskipTests")
                    .directory(new File(repoPath));

            LogSession session = new LogSession();
            Process process = pb.start();

            sessions.put(id, session);
            procs.put(id, process);

            Thread.ofVirtual().start(() -> pipeLines(process.getInputStream(), session));
            Thread.ofVirtual().start(() -> pipeLines(process.getErrorStream(), session));

            Thread.ofVirtual().start(() -> {
                try {
                    process.waitFor();
                } catch (InterruptedException ignored) {}
                lock.lock();
                try {
                    procs.remove(id);
                } finally {
                    lock.unlock();
                }
                session.closeAll();
            });
        } finally {
            lock.unlock();
        }
    }

    public void stop(String id) {
        lock.lock();
        try {
            Process process = procs.remove(id);
            if (process == null) throw new IllegalStateException("not running");
            process.descendants().forEach(ProcessHandle::destroy);
            process.destroy();
        } finally {
            lock.unlock();
        }
    }

    public boolean isRunning(String id) {
        lock.lock();
        try {
            return procs.containsKey(id);
        } finally {
            lock.unlock();
        }
    }

    public LogSession getSession(String id) {
        lock.lock();
        try {
            return sessions.get(id);
        } finally {
            lock.unlock();
        }
    }

    private void pipeLines(java.io.InputStream is, LogSession session) {
        try (BufferedReader reader = new BufferedReader(new InputStreamReader(is))) {
            String line;
            while ((line = reader.readLine()) != null) {
                session.append(line);
            }
        } catch (IOException ignored) {}
    }
}
