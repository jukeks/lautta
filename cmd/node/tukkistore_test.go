package main

import (
	"testing"

	lautta "github.com/jukeks/lautta/raft"
)

func TestStableStore(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewTukkiStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create TukkiStore: %v", err)
	}

	currentTerm := lautta.TermID(1)
	votedFor := lautta.NodeID(2)

	if err := store.Store(currentTerm, &votedFor); err != nil {
		t.Fatalf("failed to store stable state: %v", err)
	}

	restoredTerm, restoredVotedFor, err := store.Restore()
	if err != nil {
		t.Fatalf("failed to restore stable state: %v", err)
	}

	if restoredTerm != currentTerm || *restoredVotedFor != votedFor {
		t.Errorf("restored state does not match stored state: got (%d, %d), want (%d, %d)",
			restoredTerm, *restoredVotedFor, currentTerm, votedFor)
	}
}

func TestLogStore(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewTukkiStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create TukkiStore: %v", err)
	}
	if err := store.Store(2, nil); err != nil {
		t.Fatalf("failed to store stable state: %v", err)
	}
	lastLog, err := store.GetLastLog()
	if err != nil {
		t.Fatalf("failed to get last log entry: %v", err)
	}
	if lastLog.Index != 0 || lastLog.Term != 0 || len(lastLog.Payload) != 0 {
		t.Errorf("expected empty log, got %+v", lastLog)
	}

	logEntry := lautta.LogEntry{
		Index:   lastLog.Index + 1,
		Term:    lastLog.Term,
		Payload: []byte("test data"),
	}

	if err := store.Add(logEntry); err != nil {
		t.Fatalf("failed to add log entry: %v", err)
	}

	lastLog, err = store.GetLastLog()
	if err != nil {
		t.Fatalf("failed to get last log entry: %v", err)
	}
	if lastLog.Index != logEntry.Index || lastLog.Term != logEntry.Term || string(lastLog.Payload) != string(logEntry.Payload) {
		t.Errorf("last log entry does not match: got %+v, want %+v", lastLog, logEntry)
	}
	if lastLog.Index == 0 {
		t.Error("last log index should not be zero after adding an entry")
	}

	retrievedEntry, err := store.Get(logEntry.Index)
	if err != nil {
		t.Fatalf("failed to get log entry: %v", err)
	}

	if retrievedEntry.Index != logEntry.Index || retrievedEntry.Term != logEntry.Term || string(retrievedEntry.Payload) != string(logEntry.Payload) {
		t.Errorf("retrieved log entry does not match: got %+v, want %+v", retrievedEntry, logEntry)
	}

	logs, err := store.GetFrom(0)
	if err != nil {
		t.Fatalf("failed to get logs from index 0: %v", err)
	}
	if len(logs) != 1 || logs[0].Index != logEntry.Index || logs[0].Term != logEntry.Term || string(logs[0].Payload) != string(logEntry.Payload) {
		t.Errorf("logs from index 0 do not match: got %+v, want %+v", logs[0], logEntry)
	}
}
