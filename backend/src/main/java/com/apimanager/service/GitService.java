package com.apimanager.service;

import org.springframework.stereotype.Service;

import java.io.File;
import java.io.IOException;
import java.util.ArrayList;
import java.util.List;

@Service
public class GitService {

    public String currentBranch(String repoPath) throws IOException, InterruptedException {
        return run(repoPath, "git", "branch", "--show-current").strip();
    }

    public void fetch(String repoPath) throws IOException, InterruptedException {
        Result r = runWithResult(repoPath, "git", "fetch");
        if (r.exitCode() != 0) throw new IOException(r.output().strip());
    }

    public List<String> listBranches(String repoPath) throws IOException, InterruptedException {
        String out = run(repoPath, "git", "branch");
        List<String> branches = new ArrayList<>();
        for (String line : out.split("\n")) {
            String name = line.replaceFirst("^\\*?\\s+", "").strip();
            if (!name.isEmpty()) branches.add(name);
        }
        return branches;
    }

    public void checkout(String repoPath, String branch) throws IOException, InterruptedException {
        // Check for dirty working tree
        String status = run(repoPath, "git", "status", "--porcelain");
        List<String> dirty = new ArrayList<>();
        for (String line : status.split("\n")) {
            if (line.isEmpty() || line.startsWith("??")) continue;
            if (line.length() > 3) dirty.add(line.substring(3).strip());
        }
        if (!dirty.isEmpty()) throw new DirtyWorkingTreeException(dirty);

        Result r = runWithResult(repoPath, "git", "checkout", branch);
        if (r.exitCode() != 0) throw new IOException(r.output().strip());
    }

    private String run(String dir, String... cmd) throws IOException, InterruptedException {
        return runWithResult(dir, cmd).output();
    }

    private Result runWithResult(String dir, String... cmd) throws IOException, InterruptedException {
        Process process = new ProcessBuilder(cmd)
                .directory(new File(dir))
                .redirectErrorStream(true)
                .start();
        String output = new String(process.getInputStream().readAllBytes());
        int exitCode = process.waitFor();
        return new Result(exitCode, output);
    }

    public static class DirtyWorkingTreeException extends RuntimeException {
        private final List<String> files;

        public DirtyWorkingTreeException(List<String> files) {
            super("working tree has uncommitted changes");
            this.files = files;
        }

        public List<String> getFiles() { return files; }
    }

    private record Result(int exitCode, String output) {}
}
