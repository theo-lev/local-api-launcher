package com.apimanager.service;

import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import java.util.concurrent.BlockingQueue;
import java.util.concurrent.LinkedBlockingQueue;

/**
 * Buffers stdout/stderr lines for one process run and fans them out to SSE subscribers.
 * Mirrors the Go LogSession: subscribe() atomically returns a snapshot + live queue.
 */
public class LogSession {

    public static final String POISON = "\0CLOSED";

    private final List<String> lines = new ArrayList<>();
    private final Set<BlockingQueue<String>> queues = new HashSet<>();

    public synchronized void append(String line) {
        lines.add(line);
        for (BlockingQueue<String> q : queues) {
            q.offer(line); // drop on slow client rather than block
        }
    }

    /** Returns a snapshot of buffered lines and a live queue for subsequent lines. */
    public synchronized SubscribeResult subscribe() {
        BlockingQueue<String> queue = new LinkedBlockingQueue<>(256);
        List<String> snapshot = new ArrayList<>(lines);
        queues.add(queue);
        return new SubscribeResult(snapshot, queue);
    }

    public synchronized void unsubscribe(BlockingQueue<String> queue) {
        queues.remove(queue);
    }

    public synchronized void closeAll() {
        for (BlockingQueue<String> q : queues) {
            q.offer(POISON);
        }
        queues.clear();
    }

    public record SubscribeResult(List<String> snapshot, BlockingQueue<String> queue) {}
}
