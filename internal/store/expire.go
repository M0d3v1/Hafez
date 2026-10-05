package store

import (
	"context"
	"time"
)

const (
	// expireSampleSize is how many keys with a deadline one shard pass inspects.
	// Map iteration order is randomized, which is the sample.
	expireSampleSize = 20
	// expireMaxPasses stops a shard from spinning when most keys are expired.
	expireMaxPasses = 16
	// activeExpireEvery is how often RunActiveExpire samples the keyspace.
	activeExpireEvery = 100 * time.Millisecond
)

// Expire sets a deadline on key. A non-positive ttl deletes a live key.
// A missing key returns false.
func (m *Memory) Expire(key string, ttl time.Duration) bool {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	now := m.now()
	e, ok := s.alive(key, now)
	if !ok {
		return false
	}
	if ttl <= 0 {
		delete(s.data, key)
		delete(s.expires, key)
		return true
	}
	e.expire = now.Add(ttl)
	s.data[key] = e
	s.expires[key] = struct{}{}
	return true
}

// Persist clears a deadline. It returns false when the key is missing or
// already persistent.
func (m *Memory) Persist(key string) bool {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.alive(key, m.now())
	if !ok || e.expire.IsZero() {
		return false
	}
	e.expire = time.Time{}
	s.data[key] = e
	delete(s.expires, key)
	return true
}

// TTL reports the time left. exists is false for a missing key. expires is
// false when the key has no deadline.
func (m *Memory) TTL(key string) (time.Duration, bool, bool) {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	now := m.now()
	e, ok := s.alive(key, now)
	if !ok {
		return 0, false, false
	}
	if e.expire.IsZero() {
		return 0, true, false
	}
	return e.expire.Sub(now), true, true
}

// ActiveExpireCycle samples keys that have a deadline and deletes the ones
// that are due. Like Redis, a shard is sampled again when more than a quarter
// of the sample is expired.
func (m *Memory) ActiveExpireCycle() int {
	now := m.now()
	deleted := 0
	for i := range m.shards {
		s := &m.shards[i]
		for pass := 0; pass < expireMaxPasses; pass++ {
			s.mu.Lock()
			sampled, n := s.expireSample(now, expireSampleSize)
			s.mu.Unlock()
			deleted += n
			if sampled == 0 || n*4 <= sampled {
				break
			}
		}
	}
	return deleted
}

// RunActiveExpire calls ActiveExpireCycle until ctx is cancelled.
func (m *Memory) RunActiveExpire(ctx context.Context) {
	ticker := time.NewTicker(activeExpireEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.ActiveExpireCycle()
		}
	}
}

func (s *shard) expireSample(now time.Time, limit int) (sampled, deleted int) {
	if len(s.expires) == 0 || limit <= 0 {
		return 0, 0
	}
	keys := make([]string, 0, limit)
	for key := range s.expires {
		keys = append(keys, key)
		if len(keys) == limit {
			break
		}
	}
	sampled = len(keys)
	for _, key := range keys {
		e, ok := s.data[key]
		if !ok || !e.alive(now) {
			delete(s.data, key)
			delete(s.expires, key)
			deleted++
		}
	}
	return sampled, deleted
}
