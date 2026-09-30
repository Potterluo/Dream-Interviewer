package server

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Note: there is deliberately no blocking acquire on keyedMutex — see the
// type's doc comment. A blocking variant is what caused a concurrent
// duplicate to grade the NEXT question with the previous answer.

// The whole point of TryLock over Lock: a concurrent duplicate must fail
// immediately rather than queue up and then act on an interview that has
// already moved on.
func TestKeyedMutexTryLockRejectsWhileHeld(t *testing.T) {
	k := newKeyedMutex()

	release, ok := k.TryLock("iv_1")
	if !ok {
		t.Fatal("first TryLock on a free key must succeed")
	}

	if _, ok := k.TryLock("iv_1"); ok {
		t.Fatal("TryLock must fail while another holder is active")
	}
	// A different key is unaffected.
	otherRelease, ok := k.TryLock("iv_2")
	if !ok {
		t.Fatal("a different key must not be blocked")
	}
	otherRelease()

	release()
	// Free again.
	release2, ok := k.TryLock("iv_1")
	if !ok {
		t.Fatal("TryLock must succeed after the holder released")
	}
	release2()
}

func TestKeyedMutexReleaseIsIdempotent(t *testing.T) {
	k := newKeyedMutex()
	release, ok := k.TryLock("iv_1")
	if !ok {
		t.Fatal("TryLock failed")
	}
	release()
	release() // must not double-unlock or corrupt the refcount
	if _, ok := k.TryLock("iv_1"); !ok {
		t.Fatal("the key must be acquirable after a double release")
	}
}

// Entries must not accumulate: a long-running server keys by interview id,
// and leaking one mutex per interview would be a slow memory leak.
func TestKeyedMutexDropsEntries(t *testing.T) {
	k := newKeyedMutex()
	for i := 0; i < 100; i++ {
		release, ok := k.TryLock("iv_1")
		if !ok {
			t.Fatal("unexpected contention")
		}
		release()
	}
	k.mu.Lock()
	n := len(k.locks)
	k.mu.Unlock()
	if n != 0 {
		t.Fatalf("map still holds %d entries after every release", n)
	}
}

// A failed TryLock must not leave a refcount behind, or the entry could
// never be collected.
func TestKeyedMutexFailedTryLockDoesNotLeakRefs(t *testing.T) {
	k := newKeyedMutex()
	release, ok := k.TryLock("iv_1")
	if !ok {
		t.Fatal("TryLock failed")
	}
	for i := 0; i < 50; i++ {
		if _, ok := k.TryLock("iv_1"); ok {
			t.Fatal("unexpected success while held")
		}
	}
	release()
	k.mu.Lock()
	n := len(k.locks)
	k.mu.Unlock()
	if n != 0 {
		t.Fatalf("failed attempts leaked %d entries", n)
	}
}

// TryLock under real concurrency: exactly one winner per round.
func TestKeyedMutexTryLockHasExactlyOneWinner(t *testing.T) {
	k := newKeyedMutex()
	const goroutines = 16
	var winners int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			release, ok := k.TryLock("iv_1")
			if !ok {
				return
			}
			atomic.AddInt32(&winners, 1)
			time.Sleep(5 * time.Millisecond)
			release()
		}()
	}
	close(start)
	wg.Wait()
	if winners != 1 {
		t.Fatalf("winners = %d, want exactly 1", winners)
	}
}
