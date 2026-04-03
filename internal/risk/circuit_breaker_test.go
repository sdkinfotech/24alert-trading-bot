package risk

import (
	"sync"
	"testing"
	"time"
)

func TestRecordFailure_IncrementsCount(t *testing.T) {
	cb := NewCircuitBreaker(5, time.Minute)

	cb.RecordFailure()
	cb.RecordFailure()

	s := cb.State()
	if s.FailureCount != 2 {
		t.Errorf("FailureCount = %d, want 2", s.FailureCount)
	}
}

func TestIsTripped_AfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Minute)

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if !cb.IsTripped() {
		t.Error("IsTripped() should be true after reaching threshold")
	}
}

func TestIsTripped_BeforeThreshold(t *testing.T) {
	cb := NewCircuitBreaker(5, time.Minute)

	cb.RecordFailure()
	cb.RecordFailure()

	if cb.IsTripped() {
		t.Error("IsTripped() should be false before reaching threshold")
	}
}

func TestAutoReset_AfterCooldown(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	cb.RecordFailure()
	if !cb.IsTripped() {
		t.Fatal("expected tripped after failure")
	}

	time.Sleep(60 * time.Millisecond)

	if cb.IsTripped() {
		t.Error("IsTripped() should be false after cooldown expires")
	}
}

func TestManualReset(t *testing.T) {
	cb := NewCircuitBreaker(1, time.Hour)

	cb.RecordFailure()
	if !cb.IsTripped() {
		t.Fatal("expected tripped")
	}

	cb.Reset()

	if cb.IsTripped() {
		t.Error("IsTripped() should be false after Reset()")
	}

	s := cb.State()
	if s.FailureCount != 0 {
		t.Errorf("FailureCount after Reset() = %d, want 0", s.FailureCount)
	}
}

func TestRecordSuccess_ResetsFailureCount(t *testing.T) {
	cb := NewCircuitBreaker(5, time.Minute)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()

	s := cb.State()
	if s.FailureCount != 0 {
		t.Errorf("FailureCount after RecordSuccess() = %d, want 0", s.FailureCount)
	}
}

func TestState_Snapshot(t *testing.T) {
	cb := NewCircuitBreaker(3, 5*time.Minute)

	s := cb.State()
	if s.Threshold != 3 {
		t.Errorf("Threshold = %d, want 3", s.Threshold)
	}
	if s.Cooldown != 5*time.Minute {
		t.Errorf("Cooldown = %v, want 5m", s.Cooldown)
	}
	if s.Tripped {
		t.Error("new breaker should not be tripped")
	}
	if s.FailureCount != 0 {
		t.Errorf("new breaker FailureCount = %d, want 0", s.FailureCount)
	}
}

func TestState_AfterTrip(t *testing.T) {
	cb := NewCircuitBreaker(2, time.Minute)

	cb.RecordFailure()
	cb.RecordFailure()

	s := cb.State()
	if !s.Tripped {
		t.Error("State().Tripped should be true")
	}
	if s.FailureCount != 2 {
		t.Errorf("FailureCount = %d, want 2", s.FailureCount)
	}
	if s.LastFailure.IsZero() {
		t.Error("LastFailure should be set")
	}
}

func TestThreadSafety(t *testing.T) {
	cb := NewCircuitBreaker(100, time.Minute)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cb.RecordFailure()
		}()
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cb.IsTripped()
		}()
	}

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cb.State()
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cb.RecordSuccess()
		}()
	}

	wg.Wait()

	// No panic or race = pass; also ensure state is coherent
	s := cb.State()
	if s.Threshold != 100 {
		t.Errorf("Threshold = %d after concurrent access, want 100", s.Threshold)
	}
}
