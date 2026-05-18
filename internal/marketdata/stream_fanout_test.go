package marketdata

import (
	"context"
	"testing"
	"time"
)

func TestStreamFanoutPublishesToMultipleSubscribers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f := newStreamFanout[int]()
	a, err := f.subscribe(ctx, 1)
	if err != nil {
		t.Fatalf("subscribe a: %v", err)
	}
	b, err := f.subscribe(ctx, 1)
	if err != nil {
		t.Fatalf("subscribe b: %v", err)
	}

	if ok := f.publish(ctx, 42, false); !ok {
		t.Fatalf("publish returned false")
	}

	assertRecv := func(name string, ch <-chan int) {
		t.Helper()
		select {
		case got := <-ch:
			if got != 42 {
				t.Fatalf("%s got %d, want 42", name, got)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s did not receive item", name)
		}
	}

	assertRecv("a", a)
	assertRecv("b", b)
}

func TestStreamFanoutRemovesCancelledSubscriber(t *testing.T) {
	ctxA, cancelA := context.WithCancel(context.Background())
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()

	f := newStreamFanout[int]()
	if _, err := f.subscribe(ctxA, 1); err != nil {
		t.Fatalf("subscribe a: %v", err)
	}
	b, err := f.subscribe(ctxB, 1)
	if err != nil {
		t.Fatalf("subscribe b: %v", err)
	}

	cancelA()
	time.Sleep(20 * time.Millisecond)

	if ok := f.publish(ctxB, 7, false); !ok {
		t.Fatalf("publish returned false")
	}

	select {
	case got := <-b:
		if got != 7 {
			t.Fatalf("b got %d, want 7", got)
		}
	case <-time.After(time.Second):
		t.Fatalf("b did not receive item")
	}
}
