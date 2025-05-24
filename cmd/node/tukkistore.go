package main

import (
	"encoding/json"
	"fmt"
	"io"

	lautta "github.com/jukeks/lautta/raft"
	tukki "github.com/jukeks/tukki/pkg/db"
)

type TukkiStore struct {
	db *tukki.Database
}

func NewTukkiStore(dbDir string) (*TukkiStore, error) {
	store, err := tukki.OpenDatabase(dbDir, tukki.GetDefaultConfig())
	if err != nil {
		return nil, err
	}

	return &TukkiStore{
		db: store,
	}, nil
}

const (
	logPrefix      = "raft-log-"
	stableStateKey = "raft-0-stable-state"
)

type stableState struct {
	CurrentTerm lautta.TermID
	VotedFor    *lautta.NodeID
}

func logKey(index lautta.LogIndex) string {
	// need to pad with zeros to make it sortable
	return fmt.Sprintf("%s%012d", logPrefix, index)
}

func (t *TukkiStore) Store(currentTerm lautta.TermID, votedFor *lautta.NodeID) error {
	ss := stableState{
		CurrentTerm: currentTerm,
		VotedFor:    votedFor,
	}
	raw, err := json.Marshal(ss)
	if err != nil {
		return fmt.Errorf("failed to marshal stable state: %w", err)
	}
	if err := t.db.Set(stableStateKey, string(raw)); err != nil {
		return fmt.Errorf("failed to store current term: %w", err)
	}
	return nil
}

func (t *TukkiStore) Restore() (lautta.TermID, *lautta.NodeID, error) {
	raw, err := t.db.Get(stableStateKey)
	if err != nil {
		if err == tukki.ErrKeyNotFound {
			return 0, nil, nil
		}
		return 0, nil, fmt.Errorf("failed to get stable state: %w", err)
	}

	var ss stableState
	if err := json.Unmarshal([]byte(raw), &ss); err != nil {
		return 0, nil, fmt.Errorf("failed to unmarshal stable state: %w", err)
	}

	return ss.CurrentTerm, ss.VotedFor, nil

}

func (t *TukkiStore) Add(log lautta.LogEntry) error {
	key := logKey(log.Index)
	raw, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}
	if err := t.db.Set(key, string(raw)); err != nil {
		return fmt.Errorf("failed to store log entry: %w", err)
	}
	return nil
}

func (t *TukkiStore) DeleteFrom(idx lautta.LogIndex) error {
	_, err := t.db.DeleteRange(logKey(idx), "")
	return err
}

func (t *TukkiStore) Get(idx lautta.LogIndex) (lautta.LogEntry, error) {
	key := logKey(idx)
	raw, err := t.db.Get(key)
	if err != nil {
		if err == tukki.ErrKeyNotFound {
			return lautta.LogEntry{}, nil
		}
		return lautta.LogEntry{}, fmt.Errorf("failed to get log entry: %w", err)
	}

	var log lautta.LogEntry
	if err := json.Unmarshal([]byte(raw), &log); err != nil {
		return lautta.LogEntry{}, fmt.Errorf("failed to unmarshal log entry: %w", err)
	}
	return log, nil
}

func (t *TukkiStore) GetBetween(min lautta.LogIndex, max lautta.LogIndex) ([]lautta.LogEntry, error) {
	var logs []lautta.LogEntry
	cursor, err := t.db.GetCursorWithRange(logKey(min), logKey(max))
	if err != nil {
		return nil, fmt.Errorf("failed to get cursor: %w", err)
	}
	defer cursor.Close()

	for {
		pair, err := cursor.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to get next log entry: %w", err)
		}
		raw := pair.Value
		var log lautta.LogEntry
		if err := json.Unmarshal([]byte(raw), &log); err != nil {
			return nil, fmt.Errorf("failed to unmarshal log entry: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

func (t *TukkiStore) GetFrom(idx lautta.LogIndex) ([]lautta.LogEntry, error) {
	var logs []lautta.LogEntry
	cursor, err := t.db.GetCursorWithRange(logKey(idx), "")
	if err != nil {
		return nil, fmt.Errorf("failed to get cursor: %w", err)
	}
	defer cursor.Close()

	for {
		pair, err := cursor.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to get next log entry: %w", err)
		}
		raw := pair.Value
		var log lautta.LogEntry
		if err := json.Unmarshal([]byte(raw), &log); err != nil {
			return nil, fmt.Errorf("failed to unmarshal log entry: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

func (t *TukkiStore) GetLastLog() (lautta.LogEntry, error) {
	// this is slow without a reverse cursor
	cursor, err := t.db.GetCursorWithRange(logKey(0), "")
	if err != nil {
		return lautta.LogEntry{}, fmt.Errorf("failed to get cursor: %w", err)
	}
	defer cursor.Close()

	raw := ""
	for {
		pair, err := cursor.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return lautta.LogEntry{}, fmt.Errorf("failed to get next log entry: %w", err)
		}
		raw = pair.Value
	}

	if raw == "" {
		return lautta.LogEntry{}, nil
	}

	var lastLog lautta.LogEntry
	if err := json.Unmarshal([]byte(raw), &lastLog); err != nil {
		return lautta.LogEntry{}, fmt.Errorf("failed to unmarshal last log entry: %w", err)
	}

	return lastLog, nil
}
