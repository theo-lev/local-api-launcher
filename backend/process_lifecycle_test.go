package main

import (
	"strings"
	"testing"
)

func TestOldWaiterCannotCleanNewLaunch(t *testing.T) {
	store, err := NewStore(t.TempDir() + "/config.json")
	if err != nil {
		t.Fatal(err)
	}
	oldProc := &managedProcess{pid: 101, runID: "run-a", state: "stopping"}
	newProc := &managedProcess{pid: 202, runID: "run-b", state: "running"}
	pm := NewProcessManager(store)
	pm.procs["repo"] = newProc
	if err := store.StorePid("repo", ProcInfo{Pid: newProc.pid, RunID: newProc.runID}); err != nil {
		t.Fatal(err)
	}

	pm.cleanup("repo", oldProc, nil)
	if pm.procs["repo"] != newProc {
		t.Fatal("old waiter removed the newer managed process")
	}
	if info := store.GetPids()["repo"]; info.Pid != newProc.pid || info.RunID != newProc.runID {
		t.Fatalf("old waiter changed newer persisted process: %+v", info)
	}
}

func TestStartIsRejectedWhileProcessIsStopping(t *testing.T) {
	store, err := NewStore(t.TempDir() + "/config.json")
	if err != nil {
		t.Fatal(err)
	}
	pm := NewProcessManager(store)
	pm.procs["repo"] = &managedProcess{pid: 101, runID: "run-a", state: "stopping"}
	if err := pm.Start("repo", t.TempDir(), "missing-maven", "", "", "", nil); err == nil || !strings.Contains(err.Error(), "stopping") {
		t.Fatalf("Start error = %v, want stopping conflict", err)
	}
}
