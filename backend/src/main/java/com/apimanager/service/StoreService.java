package com.apimanager.service;

import com.apimanager.model.Repo;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.databind.ObjectMapper;
import jakarta.annotation.PostConstruct;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import java.io.File;
import java.io.IOException;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.locks.ReadWriteLock;
import java.util.concurrent.locks.ReentrantReadWriteLock;

@Service
public class StoreService {

    @Value("${app.config-file}")
    private String configFile;

    private final ObjectMapper mapper = new ObjectMapper();
    private final ReadWriteLock lock = new ReentrantReadWriteLock();
    private ConfigData data = new ConfigData();

    @PostConstruct
    void load() throws IOException {
        File file = new File(configFile);
        if (file.exists()) {
            data = mapper.readValue(file, ConfigData.class);
        }
    }

    public List<Repo> list() {
        lock.readLock().lock();
        try {
            return new ArrayList<>(data.repos);
        } finally {
            lock.readLock().unlock();
        }
    }

    public void add(Repo repo) throws IOException {
        lock.writeLock().lock();
        try {
            data.repos.add(repo);
            save();
        } finally {
            lock.writeLock().unlock();
        }
    }

    public boolean remove(String id) throws IOException {
        lock.writeLock().lock();
        try {
            boolean removed = data.repos.removeIf(r -> r.getId().equals(id));
            if (removed) save();
            return removed;
        } finally {
            lock.writeLock().unlock();
        }
    }

    public Repo findById(String id) {
        lock.readLock().lock();
        try {
            return data.repos.stream()
                    .filter(r -> r.getId().equals(id))
                    .findFirst()
                    .orElse(null);
        } finally {
            lock.readLock().unlock();
        }
    }

    private void save() throws IOException {
        mapper.writerWithDefaultPrettyPrinter().writeValue(new File(configFile), data);
    }

    static class ConfigData {
        @JsonProperty("repos")
        List<Repo> repos = new ArrayList<>();
    }
}
