// Package evidence manages evidence reference linking and retrieval.
package evidence

import (
	"sync"
	"time"

	"github.com/karunapuram/pathcollapse/pkg/model"
)

// Store holds evidence references indexed by source and type.
type Store struct {
	mu   sync.RWMutex
	refs []model.EvidenceRef
}

// NewStore returns an empty evidence store.
func NewStore() *Store {
	return &Store{}
}

// Add inserts a new evidence reference.
func (s *Store) Add(ref model.EvidenceRef) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refs = append(s.refs, ref)
}

// FindBySource returns all evidence references from the given source.
func (s *Store) FindBySource(source string) []model.EvidenceRef {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []model.EvidenceRef
	for _, r := range s.refs {
		if r.Source == source {
			out = append(out, r)
		}
	}
	return out
}

// FindRecent returns evidence collected after the given time.
func (s *Store) FindRecent(since time.Time) []model.EvidenceRef {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []model.EvidenceRef
	for _, r := range s.refs {
		if r.CollectedAt.After(since) {
			out = append(out, r)
		}
	}
	return out
}

// All returns all stored evidence references.
func (s *Store) All() []model.EvidenceRef {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.EvidenceRef, len(s.refs))
	copy(out, s.refs)
	return out
}
