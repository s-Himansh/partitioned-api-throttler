package throttler

import (
	"sync"
	"testing"
	"time"
)

func TestPartition_WindowFor_CreatesNew(t *testing.T) {
	p := newPartition()
	sw := p.windowFor("1.2.3.4", 10, time.Minute)

	if sw == nil {
		t.Fatal("windowFor should return non-nil")
	}

	if sw.limit != 10 {
		t.Fatalf("limit = %d, want 10", sw.limit)
	}
}

func TestPartition_WindowFor_ReturnsSame(t *testing.T) {
	p := newPartition()
	sw1 := p.windowFor("1.2.3.4", 10, time.Minute)
	sw2 := p.windowFor("1.2.3.4", 10, time.Minute)

	if sw1 != sw2 {
		t.Fatal("windowFor should return the same instance for the same IP")
	}
}

func TestPartition_WindowFor_DifferentIPs(t *testing.T) {
	p := newPartition()
	sw1 := p.windowFor("1.2.3.4", 10, time.Minute)
	sw2 := p.windowFor("5.6.7.8", 10, time.Minute)

	if sw1 == sw2 {
		t.Fatal("windowFor should return different instances for different IPs")
	}
}

func TestPartition_SnapshotLen(t *testing.T) {
	p := newPartition()

	if p.snapshotLen() != 0 {
		t.Fatal("snapshotLen should be 0 for empty partition")
	}

	p.windowFor("1.2.3.4", 10, time.Minute)
	p.windowFor("5.6.7.8", 10, time.Minute)

	if p.snapshotLen() != 2 {
		t.Fatalf("snapshotLen = %d, want 2", p.snapshotLen())
	}
}

func TestPartition_EvictIfStale(t *testing.T) {
	p := newPartition()

	sw1 := p.windowFor("1.2.3.4", 10, time.Minute)
	sw2 := p.windowFor("5.6.7.8", 10, time.Minute)
	sw1.Allow(time.Now())
	sw2.Allow(time.Now())

	// All entries are fresh (just created), nothing should be evicted
	evicted := p.evictIfStale(time.Minute)
	if evicted != 0 {
		t.Fatalf("evicted = %d, want 0", evicted)
	}

	// Evict with zero grace (all are older than zero duration)
	evicted = p.evictIfStale(0)
	if evicted != 2 {
		t.Fatalf("evicted = %d, want 2", evicted)
	}
}

func TestPartition_ConcurrentAccess(t *testing.T) {
	p := newPartition()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ip := "10.0.0.1"
			sw := p.windowFor(ip, 100, time.Minute)
			sw.Allow(time.Now())
			p.snapshotLen()
		}(i)
	}

	wg.Wait()

	if p.snapshotLen() != 1 {
		t.Fatalf("snapshotLen = %d, want 1", p.snapshotLen())
	}
}
