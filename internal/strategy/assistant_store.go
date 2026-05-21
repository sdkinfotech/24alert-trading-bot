package strategy

import (
	"sync"
	"time"
)

const assistantAnalysisTTL = 24 * time.Hour

type assistantStore struct {
	mu   sync.RWMutex
	byID map[string]*assistantEntry
}

type assistantEntry struct {
	analysis  *AssistantAnalysis
	expiresAt time.Time
}

func newAssistantStore() *assistantStore {
	return &assistantStore{byID: make(map[string]*assistantEntry)}
}

func (s *assistantStore) put(a *AssistantAnalysis) {
	if a == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[a.ID] = &assistantEntry{
		analysis:  a,
		expiresAt: time.Now().Add(assistantAnalysisTTL),
	}
	s.pruneLocked()
}

func (s *assistantStore) get(id string) (*AssistantAnalysis, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	e, ok := s.byID[id]
	if !ok || e.analysis == nil {
		return nil, false
	}
	cp := *e.analysis
	return &cp, true
}

func (s *assistantStore) delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byID, id)
}

func (s *assistantStore) pruneLocked() {
	now := time.Now()
	for id, e := range s.byID {
		if now.After(e.expiresAt) {
			delete(s.byID, id)
		}
	}
}

func (s *assistantStore) update(id string, fn func(*AssistantAnalysis)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.byID[id]
	if !ok || e.analysis == nil {
		return
	}
	fn(e.analysis)
}
