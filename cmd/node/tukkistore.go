package main

import (
	lautta "github.com/jukeks/lautta/raft"
	tukki "github.com/jukeks/tukki/pkg/db"
)

type TukkiStore struct {
	db *tukki.Database
}

func NewTukkiStore(dbDir string) (lautta.LogStore, error) {
	store, err := tukki.OpenDatabase(dbDir, tukki.GetDefaultConfig())
	if err != nil {
		return nil, err
	}

	return &TukkiStore{
		db: store,
	}, nil
}

func (t *TukkiStore) Restore() (lautta.TermID, *lautta.NodeID, error) {
	panic("unimplemented")
}

func (t *TukkiStore) Store(currentTerm lautta.TermID, votedFor *lautta.NodeID) error {
	panic("unimplemented")
}

func (t *TukkiStore) Add(lautta.LogEntry) error {
	panic("unimplemented")
}

func (t *TukkiStore) DeleteFrom(idx lautta.LogIndex) error {
	panic("unimplemented")
}

func (t *TukkiStore) Get(idx lautta.LogIndex) (lautta.LogEntry, error) {
	panic("unimplemented")
}

func (t *TukkiStore) GetBetween(min lautta.LogIndex, max lautta.LogIndex) ([]lautta.LogEntry, error) {
	panic("unimplemented")
}

func (t *TukkiStore) GetFrom(idx lautta.LogIndex) ([]lautta.LogEntry, error) {
	panic("unimplemented")
}

func (t *TukkiStore) GetLastLog() (lautta.LogEntry, error) {
	panic("unimplemented")
}
