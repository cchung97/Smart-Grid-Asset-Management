package parallel

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
)

func TestChunksN_CoversEveryIndexExactlyOnce(t *testing.T) {
	for _, tc := range []struct{ n, chunk, workers int }{
		{0, 10, 4}, {1, 10, 4}, {10, 10, 4}, {11, 10, 4}, {1000, 7, 8}, {1000, 1000, 8}, {5, 1, 3},
	} {
		hits := make([]int32, tc.n)
		err := chunksN(context.Background(), tc.n, tc.chunk, tc.workers, func(lo, hi int) {
			for i := lo; i < hi; i++ {
				atomic.AddInt32(&hits[i], 1)
			}
		})
		if err != nil {
			t.Fatalf("%+v: %v", tc, err)
		}
		for i, h := range hits {
			if h != 1 {
				t.Fatalf("%+v: index %d visited %d times", tc, i, h)
			}
		}
	}
}

func TestChunksN_PanicBecomesErrorAndDoesNotDeadlock(t *testing.T) {
	err := chunksN(context.Background(), 1000, 10, 4, func(lo, hi int) {
		if lo == 500 {
			panic("boom")
		}
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want the panic surfaced as an error", err)
	}

	err = chunksN(context.Background(), 5, 10, 4, func(lo, hi int) { panic("inline") })
	if err == nil || !strings.Contains(err.Error(), "inline") {
		t.Fatalf("inline path err = %v, want the panic surfaced as an error", err)
	}
}

func TestChunksN_HonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var processed atomic.Int32
	err := chunksN(ctx, 100000, 10, 4, func(lo, hi int) {
		if processed.Add(1) == 3 {
			cancel()
		}
	})
	if err == nil {
		t.Fatal("err = nil, want context cancellation")
	}
	if processed.Load() >= 10000 {
		t.Errorf("processed %d chunks; cancellation should stop the work early", processed.Load())
	}
}

func TestChunks_UsesDefaults(t *testing.T) {
	hits := make([]int32, 2000)
	if err := Chunks(context.Background(), len(hits), func(lo, hi int) {
		for i := lo; i < hi; i++ {
			atomic.AddInt32(&hits[i], 1)
		}
	}); err != nil {
		t.Fatal(err)
	}
	for i, h := range hits {
		if h != 1 {
			t.Fatalf("index %d visited %d times", i, h)
		}
	}
}
