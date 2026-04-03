package order

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func newTestOrder(id, accountID string, status OrderStatus) *OrderRecord {
	return &OrderRecord{
		OrderID:       id,
		AccountID:     accountID,
		InstrumentUID: "inst-001",
		Direction:     "BUY",
		OrderType:     "LIMIT",
		RequestedQty:  10,
		Status:        status,
	}
}

func TestSaveOrder_GetOrder_RoundTrip(t *testing.T) {
	repo := NewRepository()
	order := newTestOrder("order-1", "acc-1", OrderStatusNew)

	repo.SaveOrder(order)

	got, err := repo.GetOrder("order-1")
	if err != nil {
		t.Fatalf("GetOrder() error: %v", err)
	}
	if got.OrderID != "order-1" {
		t.Errorf("OrderID = %q, want %q", got.OrderID, "order-1")
	}
	if got.AccountID != "acc-1" {
		t.Errorf("AccountID = %q, want %q", got.AccountID, "acc-1")
	}
	if got.Status != OrderStatusNew {
		t.Errorf("Status = %q, want %q", got.Status, OrderStatusNew)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set after SaveOrder")
	}
}

func TestGetOrder_NotFound(t *testing.T) {
	repo := NewRepository()
	_, err := repo.GetOrder("nonexistent")
	if err == nil {
		t.Error("GetOrder() should return error for nonexistent order")
	}
}

func TestGetOrder_ReturnsCopy(t *testing.T) {
	repo := NewRepository()
	repo.SaveOrder(newTestOrder("order-1", "acc-1", OrderStatusNew))

	got, _ := repo.GetOrder("order-1")
	got.Status = OrderStatusCancelled

	original, _ := repo.GetOrder("order-1")
	if original.Status != OrderStatusNew {
		t.Error("GetOrder() should return a copy, not a reference to internal data")
	}
}

func TestGetActiveOrders(t *testing.T) {
	repo := NewRepository()
	repo.SaveOrder(newTestOrder("o1", "acc-1", OrderStatusNew))
	repo.SaveOrder(newTestOrder("o2", "acc-1", OrderStatusPartiallyFilled))
	repo.SaveOrder(newTestOrder("o3", "acc-1", OrderStatusFilled))
	repo.SaveOrder(newTestOrder("o4", "acc-1", OrderStatusCancelled))
	repo.SaveOrder(newTestOrder("o5", "acc-1", OrderStatusRejected))
	repo.SaveOrder(newTestOrder("o6", "acc-2", OrderStatusNew))

	active := repo.GetActiveOrders("acc-1")
	if len(active) != 2 {
		t.Errorf("GetActiveOrders(acc-1) returned %d orders, want 2", len(active))
	}

	for _, o := range active {
		if o.Status == OrderStatusFilled || o.Status == OrderStatusCancelled || o.Status == OrderStatusRejected {
			t.Errorf("GetActiveOrders returned terminal order: %s status=%s", o.OrderID, o.Status)
		}
		if o.AccountID != "acc-1" {
			t.Errorf("GetActiveOrders returned order from wrong account: %s", o.AccountID)
		}
	}
}

func TestGetActiveOrders_Empty(t *testing.T) {
	repo := NewRepository()
	active := repo.GetActiveOrders("no-such-account")
	if len(active) != 0 {
		t.Errorf("GetActiveOrders for unknown account returned %d, want 0", len(active))
	}
}

func TestUpdateOrderState(t *testing.T) {
	repo := NewRepository()
	repo.SaveOrder(newTestOrder("order-1", "acc-1", OrderStatusNew))

	err := repo.UpdateOrderState("order-1", OrderStatusFilled, 10)
	if err != nil {
		t.Fatalf("UpdateOrderState() error: %v", err)
	}

	got, _ := repo.GetOrder("order-1")
	if got.Status != OrderStatusFilled {
		t.Errorf("Status after update = %q, want %q", got.Status, OrderStatusFilled)
	}
	if got.FilledQty != 10 {
		t.Errorf("FilledQty = %d, want 10", got.FilledQty)
	}
}

func TestUpdateOrderState_NotFound(t *testing.T) {
	repo := NewRepository()
	err := repo.UpdateOrderState("nope", OrderStatusFilled, 1)
	if err == nil {
		t.Error("UpdateOrderState() should return error for nonexistent order")
	}
}

func TestAddExecution_GetJournal(t *testing.T) {
	repo := NewRepository()

	exec1 := ExecutionRecord{
		OrderID:    "order-1",
		TradeID:    "trade-1",
		Qty:        5,
		Price:      100.5,
		ExecutedAt: time.Now(),
	}
	exec2 := ExecutionRecord{
		OrderID:    "order-1",
		TradeID:    "trade-2",
		Qty:        3,
		Price:      101.0,
		ExecutedAt: time.Now(),
	}

	repo.AddExecution(exec1)
	repo.AddExecution(exec2)

	journal := repo.GetJournal()
	if len(journal) != 2 {
		t.Fatalf("GetJournal() returned %d entries, want 2", len(journal))
	}
	if journal[0].TradeID != "trade-1" {
		t.Errorf("journal[0].TradeID = %q, want %q", journal[0].TradeID, "trade-1")
	}
	if journal[1].TradeID != "trade-2" {
		t.Errorf("journal[1].TradeID = %q, want %q", journal[1].TradeID, "trade-2")
	}
}

func TestGetJournal_ReturnsCopy(t *testing.T) {
	repo := NewRepository()
	repo.AddExecution(ExecutionRecord{TradeID: "t1"})

	j1 := repo.GetJournal()
	j1[0].TradeID = "modified"

	j2 := repo.GetJournal()
	if j2[0].TradeID != "t1" {
		t.Error("GetJournal() should return a copy")
	}
}

func TestConcurrentAccess(t *testing.T) {
	repo := NewRepository()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			repo.SaveOrder(newTestOrder(fmt.Sprintf("order-%d", i), "acc-1", OrderStatusNew))
		}(i)
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			repo.GetOrder(fmt.Sprintf("order-%d", i))
		}(i)
	}

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			repo.GetActiveOrders("acc-1")
		}()
	}

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			repo.AddExecution(ExecutionRecord{
				OrderID: fmt.Sprintf("order-%d", i),
				TradeID: fmt.Sprintf("trade-%d", i),
			})
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			repo.GetJournal()
		}()
	}

	wg.Wait()
}
