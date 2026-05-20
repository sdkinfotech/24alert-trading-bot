package tradeanalyst

import (
	"log/slog"
	"sync"
)

// Service runs post-market analysis and persists results.
type Service struct {
	store *Store
	log   *slog.Logger
	mu    sync.Mutex
}

// NewService opens the analyst store.
func NewService(log *slog.Logger) (*Service, error) {
	st, err := OpenStore()
	if err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{store: st, log: log}, nil
}

// DefaultLogger is used when runner passes nil.
func DefaultLogger() *slog.Logger { return slog.Default() }

func (s *Service) Store() *Store { return s.store }

// RunPostMarket analyzes a session, saves report + hints.
func (s *Service) RunPostMarket(in SessionInput) (*SessionReport, error) {
	journal, err := LoadJournalEvents(s.store.JournalPath(), in.SessionID)
	if err != nil {
		s.log.Warn("trade analyst: journal read", "session", in.SessionID, "error", err)
		journal = nil
	}
	rep, err := AnalyzeSession(in, journal)
	if err != nil {
		return nil, err
	}
	if err := s.store.SaveReport(rep); err != nil {
		return nil, err
	}
	if h := HintsFromReport(rep); h != nil {
		_ = s.store.SaveHints(h)
	}
	s.log.Info("trade analyst post-market done",
		"session", in.SessionID, "ticker", in.Ticker, "rounds", len(rep.TradeRounds))
	return rep, nil
}
