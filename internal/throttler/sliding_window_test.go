package throttler

import (
	"sync"
	"testing"
	"time"
)

func TestSlidingWindow_AllowWithinLimit(t *testing.T) {
	sw := newSlidingWindow(5, time.Minute)
	now := time.Now()

	for i := 0; i < 5; i++ {
		if !sw.Allow(now.Add(time.Duration(i) * time.Second)) {
			t.Fatalf("request %d should be allowed", i)
		}
	}
}

func TestSlidingWindow_DenyOverLimit(t *testing.T) {
	sw := newSlidingWindow(3, time.Minute)
	now := time.Now()

	for i := 0; i < 3; i++ {
		sw.Allow(now)
	}

	if sw.Allow(now) {
		t.Fatal("should be denied after exceeding limit")
	}
}

func TestSlidingWindow_WindowExpiry(t *testing.T) {
	sw := newSlidingWindow(2, time.Second)
	now := time.Now()

	sw.Allow(now)
	sw.Allow(now)

	if sw.Allow(now) {
		t.Fatal("should be denied at limit")
	}

	if !sw.Allow(now.Add(2 * time.Second)) {
		t.Fatal("should be allowed after window expires")
	}
}

func TestSlidingWindow_LastSeen(t *testing.T) {
	sw := newSlidingWindow(5, time.Minute)

	if !sw.LastSeen().IsZero() {
		t.Fatal("LastSeen should be zero for empty window")
	}

	now := time.Now()
	sw.Allow(now)

	if sw.LastSeen() != now {
		t.Fatalf("LastSeen = %v, want %v", sw.LastSeen(), now)
	}
}

func TestSlidingWindow_ConcurrentAccess(t *testing.T) {
	sw := newSlidingWindow(1000, time.Minute)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				sw.Allow(time.Now())
			}
		}()
	}

	wg.Wait()

	if sw.LastSeen().IsZero() {
		t.Fatal("LastSeen should not be zero after concurrent access")
	}
}

func TestSlidingWindow_CompactsOldEntries(t *testing.T) {
	sw := newSlidingWindow(10, time.Second)
	now := time.Now()

	for i := 0; i < 5; i++ {
		sw.Allow(now.Add(time.Duration(i) * 100 * time.Millisecond))
	}

	sw.mu.Lock()
	before := len(sw.timestamps)
	sw.mu.Unlock()

	sw.Allow(now.Add(2 * time.Second))

	sw.mu.Lock()
	after := len(sw.timestamps)
	sw.mu.Unlock()

	if after >= before {
		t.Fatalf("expected compaction: before=%d after=%d", before, after)
	}
}
