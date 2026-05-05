package com.apimanager.model;

import com.fasterxml.jackson.annotation.JsonInclude;

@JsonInclude(JsonInclude.Include.NON_DEFAULT)
public class RepoInfo {

    private String id;
    private String path;
    private String currentBranch;
    private String status;
    private int port;
    private boolean pathError;

    public String getId() { return id; }
    public void setId(String id) { this.id = id; }

    public String getPath() { return path; }
    public void setPath(String path) { this.path = path; }

    public String getCurrentBranch() { return currentBranch; }
    public void setCurrentBranch(String currentBranch) { this.currentBranch = currentBranch; }

    public String getStatus() { return status; }
    public void setStatus(String status) { this.status = status; }

    public int getPort() { return port; }
    public void setPort(int port) { this.port = port; }

    public boolean isPathError() { return pathError; }
    public void setPathError(boolean pathError) { this.pathError = pathError; }
}
