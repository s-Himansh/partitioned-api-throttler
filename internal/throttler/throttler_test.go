package throttler

import (
	"sync"
	"testing"
	"time"
)

func TestThrottler_AllowBasic(t *testing.T) {
	th := New(4, 3, time.Minute)

	for i := 0; i < 3; i++ {
		if !th.Allow("1.2.3.4", "/api/test") {
			t.Fatalf("request %d should be allowed", i)
		}
	}

	if th.Allow("1.2.3.4", "/api/test") {
		t.Fatal("should be denied after limit")
	}
}

func TestThrottler_DifferentIPsAreIndependent(t *testing.T) {
	th := New(4, 2, time.Minute)

	th.Allow("1.1.1.1", "/api/test")
	th.Allow("1.1.1.1", "/api/test")

	if th.Allow("1.1.1.1", "/api/test") {
		t.Fatal("IP1 should be denied")
	}

	if !th.Allow("2.2.2.2", "/api/test") {
		t.Fatal("IP2 should be allowed")
	}
}

func TestThrottler_WindowExpiry(t *testing.T) {
	th := New(4, 1, time.Millisecond)

	th.Allow("1.2.3.4", "/api/test")

	time.Sleep(5 * time.Millisecond)

	if !th.Allow("1.2.3.4", "/api/test") {
		t.Fatal("should be allowed after window expires")
	}
}

func TestThrottler_PartitionDistribution(t *testing.T) {
	th := New(4, 100, time.Minute)

	seen := make(map[uint32]bool)
	for i := 0; i < 1000; i++ {
		idx := th.PartitionIndex("10.0.0." + string(rune('0'+i%10)))
		seen[idx] = true
	}

	if len(seen) < 2 {
		t.Fatalf("expected multiple partitions to be used, got %d", len(seen))
	}
}

func TestThrottler_MetricsTracked(t *testing.T) {
	th := New(4, 2, time.Minute)

	th.Allow("1.2.3.4", "/api/test")
	th.Allow("1.2.3.4", "/api/test")
	th.Allow("1.2.3.4", "/api/test") // denied

	allowed := th.totalAllowed.Load()
	denied := th.totalDenied.Load()

	if allowed != 2 {
		t.Fatalf("totalAllowed = %d, want 2", allowed)
	}

	if denied != 1 {
		t.Fatalf("totalDenied = %d, want 1", denied)
	}
}

func TestThrottler_ConcurrentAllow(t *testing.T) {
	th := New(16, 100, time.Minute)
	var wg sync.WaitGroup

	allowed := make(chan bool, 1000)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				allowed <- th.Allow("1.2.3.4", "/api/test")
			}
		}()
	}

	wg.Wait()
	close(allowed)

	count := 0
	for a := range allowed {
		if a {
			count++
		}
	}

	if count != 100 {
		t.Fatalf("allowed = %d, want 100 (limit)", count)
	}
}

func TestThrottler_ZeroPartitions(t *testing.T) {
	th := New(0, 10, time.Minute)

	if th.numPartitions != 64 {
		t.Fatalf("numPartitions = %d, want 64 (default)", th.numPartitions)
	}
}

func TestThrottler_NegativePartitions(t *testing.T) {
	th := New(-1, 10, time.Minute)

	if th.numPartitions != 64 {
		t.Fatalf("numPartitions = %d, want 64 (default)", th.numPartitions)
	}
}
