package server

import "sync"

// keyedMutex serialises work per key, with entries removed when the last
// holder releases so the map cannot grow without bound over a long uptime.
//
// Why this exists: the store's compare-and-set stops a duplicated `answer`
// from being WRITTEN twice, but it does not stop the duplicate from being
// processed — both requests reach the model before either write happens,
// so the candidate pays for two gradings.
//
// Why it is TryLock ONLY: blocking the loser is actively harmful here. By
// the time it acquired the lock the winner would have advanced the
// interview, so the loser would find a DIFFERENT open question and
// legitimately write the stale submission onto it — the CAS cannot catch
// that, because it is a different row. That regression was observed in
// practice, so this type deliberately offers no blocking acquire: the API
// makes the mistake unavailable.
type keyedMutex struct {
	mu    sync.Mutex
	locks map[string]*keyedLock
}

type keyedLock struct {
	mu   sync.Mutex
	refs int
}

func newKeyedMutex() *keyedMutex {
	return &keyedMutex{locks: map[string]*keyedLock{}}
}

// TryLock acquires the key only if it is free right now. It returns
// (release, true) on success and (no-op, false) when another holder is
// active, without waiting for it.
func (k *keyedMutex) TryLock(key string) (func(), bool) {
	entry := k.acquire(key)
	if !entry.mu.TryLock() {
		// Never held it, so the release must not unlock it.
		k.release(key, entry, false)
		return func() {}, false
	}
	var once sync.Once
	return func() { once.Do(func() { k.release(key, entry, true) }) }, true
}

// acquire registers interest in key so the entry cannot be collected
// while this caller is still deciding what to do with it.
func (k *keyedMutex) acquire(key string) *keyedLock {
	k.mu.Lock()
	defer k.mu.Unlock()
	entry := k.locks[key]
	if entry == nil {
		entry = &keyedLock{}
		k.locks[key] = entry
	}
	entry.refs++
	return entry
}

// release drops the refcount, unlocking the entry only when this caller
// actually held it.
func (k *keyedMutex) release(key string, entry *keyedLock, held bool) {
	if held {
		entry.mu.Unlock()
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	entry.refs--
	if entry.refs == 0 {
		delete(k.locks, key)
	}
}
