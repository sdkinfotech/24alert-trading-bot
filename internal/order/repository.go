package order

import (
	"fmt"
	"sync"
	"time"
)

type OrderStatus string

const (
	OrderStatusNew             OrderStatus = "NEW"
	OrderStatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"
	OrderStatusFilled          OrderStatus = "FILLED"
	OrderStatusCancelled       OrderStatus = "CANCELLED"
	OrderStatusRejected        OrderStatus = "REJECTED"
	OrderStatusReplaced        OrderStatus = "REPLACED"
)

type OrderRecord struct {
	OrderID       string
	AccountID     string
	InstrumentUID string
	Direction     string
	OrderType     string
	RequestedQty  int64
	FilledQty     int64
	Status        OrderStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ExecutionRecord struct {
	OrderID    string
	TradeID    string
	Qty        int64
	Price      float64
	ExecutedAt time.Time
}

type Repository struct {
	mu           sync.RWMutex
	activeOrders map[string]*OrderRecord
	journal      []ExecutionRecord
}

func NewRepository() *Repository {
	return &Repository{
		activeOrders: make(map[string]*OrderRecord),
	}
}

func (r *Repository) SaveOrder(rec *OrderRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	rec.CreatedAt = now
	rec.UpdatedAt = now
	r.activeOrders[rec.OrderID] = rec
}

func (r *Repository) GetOrder(orderID string) (*OrderRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rec, ok := r.activeOrders[orderID]
	if !ok {
		return nil, fmt.Errorf("order %s not found", orderID)
	}
	cp := *rec
	return &cp, nil
}

func (r *Repository) GetActiveOrders(accountID string) []*OrderRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*OrderRecord
	for _, rec := range r.activeOrders {
		if rec.AccountID == accountID && rec.Status != OrderStatusFilled &&
			rec.Status != OrderStatusCancelled && rec.Status != OrderStatusRejected {
			cp := *rec
			result = append(result, &cp)
		}
	}
	return result
}

func (r *Repository) UpdateOrderState(orderID string, status OrderStatus, filledQty int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	rec, ok := r.activeOrders[orderID]
	if !ok {
		return fmt.Errorf("order %s not found", orderID)
	}
	rec.Status = status
	rec.FilledQty = filledQty
	rec.UpdatedAt = time.Now()
	return nil
}

func (r *Repository) AddExecution(exec ExecutionRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.journal = append(r.journal, exec)
}

func (r *Repository) GetJournal() []ExecutionRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ExecutionRecord, len(r.journal))
	copy(result, r.journal)
	return result
}
